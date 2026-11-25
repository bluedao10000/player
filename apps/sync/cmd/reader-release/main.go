// reader-release moves one accepted, explicitly opted-in episode at a time.
// send runs on the archive host; receive is an SSH forced command on the reader host.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"image/jpeg"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"

	queries "anime-agent/api/internal/repository/queries/release"
)

const limit = 16 << 20

var sceneImage = regexp.MustCompile(`^/art/episode-scenes/([0-9a-f]{64})\.jpg$`)
var slug = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

type envelope struct {
	Payload json.RawMessage
	Image   []byte
}
type pick struct {
	Chapter, Anchor, Image, Block string
	Slot, Round, Variant          int
}
type block struct {
	Key, Text string
	Anchors   []string
}
type acceptance struct {
	ID                 int64
	Topic, Fingerprint string
	World              string `json:"edition_slug"`
	Passed             bool
	First              int `json:"scope_first"`
	Last               int `json:"scope_last"`
	Checks             []struct {
		Name  string
		Items []json.RawMessage
	}
	Picks []pick
	Copy  map[string][]block `json:"reader_copy"`
}

func (e envelope) inspect() (acceptance, string, error) {
	var rows map[string][]json.RawMessage
	var a acceptance
	if err := json.Unmarshal(e.Payload, &rows); err != nil {
		return a, "", err
	}
	for _, table := range []string{"versions", "research_topics", "research_runs", "episode_extensions", "episode_plans", "episode_art_jobs", "media_candidates", "scope_acceptances"} {
		if len(rows[table]) != 1 {
			return a, "", fmt.Errorf("expected one %s", table)
		}
	}
	if err := json.Unmarshal(rows["scope_acceptances"][0], &a); err != nil {
		return a, "", err
	}
	if !a.Passed || len(a.Picks) != 1 || len(a.Checks) != 5 || !slug.MatchString(a.World) || a.First != a.Last || a.First < 1 || a.Topic != fmt.Sprintf("%s-ep-%03d", a.World, a.First) || a.Fingerprint == "" || a.ID < 1 {
		return a, "", errors.New("invalid episode acceptance")
	}
	required := map[string]bool{"人物": true, "剧情": true, "世界观": true, "时间线": true, "图片": true}
	for _, check := range a.Checks {
		if !required[check.Name] || len(check.Items) == 0 {
			return a, "", errors.New("incomplete acceptance dimension")
		}
		delete(required, check.Name)
	}
	p := a.Picks[0]
	if !slug.MatchString(p.Chapter) || !slug.MatchString(p.Anchor) || len(a.Copy[p.Chapter]) == 0 {
		return a, "", errors.New("invalid chapter or scene")
	}
	anchored := false
	for _, b := range a.Copy[p.Chapter] {
		if b.Key == "" || b.Text == "" {
			return a, "", errors.New("empty passage")
		}
		for _, anchor := range b.Anchors {
			if b.Key == p.Block && anchor == "passage-"+p.Anchor {
				anchored = true
			}
		}
	}
	if !anchored {
		return a, "", errors.New("selected scene lacks passage")
	}
	m := sceneImage.FindStringSubmatch(p.Image)
	if m == nil {
		return a, "", errors.New("invalid image address")
	}
	return a, m[1], nil
}
func digest(b []byte) string { return fmt.Sprintf("%x", sha256.Sum256(b)) }
func installImage(dir, sha string, data []byte) error {
	if digest(data) != sha {
		return errors.New("selected image checksum mismatch")
	}
	if _, err := jpeg.DecodeConfig(bytes.NewReader(data)); err != nil {
		return err
	}
	dest := filepath.Join(dir, sha)
	f, err := os.CreateTemp(dir, ".release-")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err == nil {
		err = f.Chmod(0644)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err = os.Link(f.Name(), dest); errors.Is(err, os.ErrExist) {
		b, err := os.ReadFile(dest)
		if err != nil {
			return err
		}
		if digest(b) != sha {
			return errors.New("archive collision")
		}
	} else if err != nil {
		return err
	}
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}

// Destination IDs differ from the production ledger. The original acceptance ID
// and its immutable fingerprint identify a repeated delivery after a lost response.
func receive(ctx context.Context, db *pgx.Conn, in io.Reader) (int64, error) {
	raw, err := io.ReadAll(io.LimitReader(in, limit+1))
	if err != nil || len(raw) > limit {
		return 0, errors.New("invalid transfer size")
	}
	var e envelope
	if err = json.Unmarshal(raw, &e); err != nil {
		return 0, err
	}
	a, sha, err := e.inspect()
	if err != nil {
		return 0, err
	}
	var id int64
	err = pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, queries.LockPublication); err != nil {
			return err
		}
		var valid bool
		err := tx.QueryRow(ctx, queries.ValidateRelease, e.Payload, a.World, a.First, a.Picks[0].Chapter, a.Picks[0].Anchor, a.Topic).Scan(&valid)
		if err != nil {
			return err
		}
		if !valid {
			return errors.New("edition, selected media, or reusable portraits differ")
		}
		var origin int64
		var fp string
		err = tx.QueryRow(ctx, queries.FindReceipt, a.Topic).Scan(&origin, &fp, &id)
		if err == nil && (origin != a.ID || fp != a.Fingerprint) {
			return errors.New("different final already delivered")
		}
		fresh := errors.Is(err, pgx.ErrNoRows)
		if err != nil && !fresh {
			return err
		}
		if fresh {
			var collision bool
			err = tx.QueryRow(ctx, queries.HasChapterConflict, a.World, a.Picks[0].Chapter, a.Picks[0].Anchor, a.First).Scan(&collision)
			if err != nil {
				return err
			}
			if collision {
				return errors.New("existing chapter or scene link")
			}
			if err = tx.QueryRow(ctx, queries.ImportRelease, e.Payload).Scan(&id); err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, queries.InsertReceipt, a.Topic, a.ID, a.Fingerprint, id); err != nil {
				return err
			}
		}
		if err = tx.QueryRow(ctx, queries.VerifyInstalledScope, a.Topic, id, e.Payload, a.Picks[0].Image).Scan(&valid); err != nil {
			return err
		}
		if !valid {
			return errors.New("chapter not visible with its predecessors")
		}
		if err := installImage(os.Getenv("KNOWLEDGE_ARCHIVE_DIR"), sha, e.Image); err != nil {
			return err
		}
		return publishImage(ctx, sha, e.Image)
	})
	return id, err
}

func main() {
	if len(os.Args) != 2 {
		log.Fatal("expected send or receive")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	db, err := pgx.Connect(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal("release database connection failed")
	}
	defer db.Close(context.Background())
	switch os.Args[1] {
	case "send":
		err = send(ctx, db)
	case "receive":
		var id int64
		id, err = receive(ctx, db, os.Stdin)
		if err == nil {
			fmt.Println(id)
		}
	default:
		err = errors.New("unknown command")
	}
	if err != nil {
		log.Fatal(err)
	}
}
