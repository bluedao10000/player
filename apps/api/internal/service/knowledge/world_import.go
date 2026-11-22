package knowledge

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"

	queries "anime-agent/api/internal/repository/queries/knowledge"
)

//go:embed worlddata/*.json
var worldFiles embed.FS

type worldProof struct{ URI, Locator, Digest string }
type worldPerson struct {
	Slug, Name, Role, Portrait, Href, Alias, OriginalName string
	ProviderID                                            int `json:"provider_id"`
}
type worldScene struct {
	Slug, Name, Place, Block, Text string
	Cast                           []string
}
type worldBlock struct{ Key, Text string }
type worldChapter struct {
	Slug, Name, Label, Intro, Href, Image, OriginalName string
	Episode, ProviderID                                 int
	Cast                                                []string
	Blocks                                              []worldBlock
	Scenes                                              []worldScene
}
type worldLine struct {
	Slug, Title, Intro string
	Chapters           []string
}
type worldManifest struct {
	Schema, Position                  int
	Slug, Name, Edition, Intro, Image string
	CastTitle                         string `json:"cast_title"`
	People                            []worldPerson
	Lines                             []worldLine
	Chapters                          []worldChapter
	Proofs                            map[string]worldProof
}

var worldSlug = regexp.MustCompile("^[a-z0-9]+(-[a-z0-9]+)*$")

func digest(b []byte) string { return fmt.Sprintf("sha256:%x", sha256.Sum256(b)) }

func loadWorld(ctx context.Context, conn *pgx.Conn, s source, st *stats) (worldManifest, error) {
	var w worldManifest
	data, err := worldFiles.ReadFile("worlddata/" + s.Asset)
	if err != nil {
		return w, err
	}
	if err = json.Unmarshal(data, &w); err != nil {
		return w, err
	}
	w.Proofs = map[string]worldProof{"editorial": {URI: "local:worlddata/" + s.Asset, Locator: "manifest", Digest: digest(data)}}
	if s.Kind == "world-enrichment" {
		fetcher := worldFetcher{conn: conn, sourceID: s.ID, changed: make(map[string]cachedWorldDocument)}
		if err = enrichBebop(ctx, s, &w, st, &fetcher); err != nil {
			return w, err
		}
		for url, doc := range fetcher.changed {
			_, err = conn.Exec(ctx, queries.CacheWorldPage, s.ID, url, doc.etag, doc.modified, doc.body)
			if err != nil {
				return w, err
			}
		}
	}
	return w, err
}

func (w worldManifest) validate() error {
	if w.Schema != 1 || !worldSlug.MatchString(w.Slug) || w.Name == "" || w.Position < 1 || len(w.People) == 0 || len(w.Chapters) == 0 || len(w.Lines) == 0 {
		return errors.New("incomplete world manifest")
	}
	for _, p := range w.People {
		if !worldSlug.MatchString(p.Slug) || p.Name == "" || (p.Href != "" && p.Href != "/character/"+p.Slug) {
			return fmt.Errorf("invalid character %q", p.Slug)
		}
	}
	for _, c := range w.Chapters {
		if !worldSlug.MatchString(c.Slug) || c.Name == "" || len(c.Blocks) == 0 || len(c.Cast) == 0 || (c.Href != "" && c.Href != "/story/"+c.Slug) {
			return fmt.Errorf("invalid chapter %q", c.Slug)
		}
		for _, b := range c.Blocks {
			if !worldSlug.MatchString(b.Key) || strings.TrimSpace(b.Text) == "" {
				return fmt.Errorf("invalid block in %s", c.Slug)
			}
		}
		for _, sc := range c.Scenes {
			if !worldSlug.MatchString(sc.Slug) || sc.Name == "" || sc.Text == "" || sc.Place == "" {
				return fmt.Errorf("invalid scene in %s", c.Slug)
			}
		}
	}
	return nil
}

func importWorld(ctx context.Context, conn *pgx.Conn, s source, run string, st *stats, replay string) error {
	if s.Kind == "world-enrichment" && replay == "" {
		var deferred bool
		if err := conn.QueryRow(ctx, queries.SourceInBackoff, s.ID).Scan(&deferred); err != nil {
			return err
		}
		if deferred {
			_, err := conn.Exec(ctx, queries.SkipRateLimitedWorld, run)
			return err
		}
	}
	var w worldManifest
	var err error
	if replay != "" {
		var raw []byte
		err = conn.QueryRow(ctx, queries.WorldManifest, s.ID, replay, s.Local).Scan(&raw)
		if err == nil {
			err = json.Unmarshal(raw, &w)
		}
	} else {
		w, err = loadWorld(ctx, conn, s, st)
	}
	if err != nil {
		return err
	}
	if err = w.validate(); err != nil {
		return err
	}
	data, err := json.Marshal(w)
	if err != nil {
		return err
	}
	st.Remote = digest(data)
	st.Checksum = st.Remote
	st.URL = s.DataURL
	st.Read = 1
	st.Length = int64(len(data))
	if st.Remote == s.Local {
		_, err = conn.Exec(ctx, queries.SkipUnchangedWorld, run, st.Remote)
		return err
	}
	// One staging row represents the whole work, like one upstream work record.
	_, err = conn.Exec(ctx, queries.StageWorld, run, w.Slug, data)
	if err != nil {
		return err
	}
	err = pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
		if _, e := tx.Exec(ctx, queries.LockProjection); e != nil {
			return e
		}
		var v int64
		e := tx.QueryRow(ctx, queries.RegisterWorldVersion, s.ID, st.Remote, s.Kind, s.Asset, run, w.Slug).Scan(&v)
		if e != nil {
			return e
		}
		work, e := resolveWorld(ctx, tx, w, s, v)
		if e != nil {
			return e
		}
		var inserted int
		e = tx.QueryRow(ctx, queries.PublishWorldRelease, v, work).Scan(&inserted)
		if e != nil && !errors.Is(e, pgx.ErrNoRows) {
			return e
		}
		if inserted == 1 {
			if e = storeWorldGraph(ctx, tx, w, v, s.Kind == "world-enrichment"); e != nil {
				return e
			}
		}
		origin := "original"
		if s.Kind == "world-enrichment" {
			origin = "external"
		}
		_, e = tx.Exec(ctx, queries.SaveWorld, work, w.Slug, v, w.Name, w.Edition, w.Intro, w.CastTitle, w.Image, origin)
		if e != nil {
			return e
		}
		if _, e = tx.Exec(ctx, queries.SeedImportedWorldMediaBatchWorlds); e != nil {
			return e
		}
		_, e = tx.Exec(ctx, queries.SetHomeSlot, w.Position, work)
		if e != nil {
			return e
		}
		_, e = tx.Exec(ctx, queries.AdvanceWorldVersion, s.ID, st.Remote)
		if e != nil {
			return e
		}
		_, e = tx.Exec(ctx, queries.DeleteBatch, run)
		if e != nil {
			return e
		}
		_, e = tx.Exec(ctx, queries.FinishWorldImport, run, st.Remote, st.URL, st.Length, st.HTTP)
		return e
	})
	if err != nil {
		st.Failed = 1
	}
	return err
}

func resolveWorld(ctx context.Context, tx pgx.Tx, w worldManifest, s source, v int64) (string, error) {
	var id string
	if s.Kind == "world-enrichment" {
		err := tx.QueryRow(ctx, queries.FindTVEdition).Scan(&id)
		if err != nil {
			return "", fmt.Errorf("resolve unique 1998 TV subject 253: %w", err)
		}
		return id, nil
	}
	err := tx.QueryRow(ctx, queries.LinkedWorldWork, s.ID, w.Slug).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		err = tx.QueryRow(ctx, queries.CreateOriginalWork, w.Name, w.Intro).Scan(&id)
	}
	if err != nil {
		return "", err
	}
	norm := normalized{Title: w.Name, Original: w.Name, Type: "SHORT_STORY_COLLECTION", Synopsis: w.Intro, Titles: []title{{Lang: "zh", Title: w.Name}}}
	_, err = tx.Exec(ctx, queries.UpsertWorldRecord, s.ID, w.Slug, id, v, norm)
	if err != nil {
		return "", err
	}
	if _, err = tx.Exec(ctx, queries.CreateAffectedWorks); err != nil {
		return "", err
	}
	if _, err = tx.Exec(ctx, queries.MarkAffectedWork, id); err != nil {
		return "", err
	}
	_, err = tx.Exec(ctx, queries.Projection)
	return id, err
}

func storeWorldGraph(ctx context.Context, tx pgx.Tx, w worldManifest, v int64, external bool) error {
	b := &pgx.Batch{}
	for k, p := range w.Proofs {
		b.Queue(queries.InsertEvidence, v, k, p.URI, p.Locator, p.Digest)
	}
	for i, p := range w.People {
		proof := "editorial"
		if external {
			proof = "character/" + p.Slug
		}
		b.Queue(queries.InsertCharacter, v, p.Slug, p.Name, p.Role, p.Portrait, p.Href, i, p.ProviderID, p.OriginalName, proof)
	}
	for i, c := range w.Chapters {
		proof, identity := "editorial", "editorial"
		if external {
			proof = "episode/" + c.Slug
			identity = "catalog/" + c.Slug
		}
		b.Queue(queries.InsertChapter, v, c.Slug, c.Name, c.Label, c.Intro, c.Href, c.Image, i, c.Episode, c.ProviderID, c.OriginalName, identity)
		for j, p := range c.Blocks {
			b.Queue(queries.InsertBlock, v, c.Slug, p.Key, p.Text, j)
		}
		for _, p := range c.Cast {
			appearance := proof
			if external && c.Episode == 2 && (p == "jet" || p == "hakim") {
				appearance = "chase-account"
			}
			b.Queue(queries.InsertAppearance, v, c.Slug, p, appearance)
		}
		for j, sc := range c.Scenes {
			b.Queue(queries.InsertSetting, v, sc.Place)
			b.Queue(queries.InsertScene, v, sc.Slug, c.Slug, sc.Name, sc.Place, sc.Text, sc.Block, j, proof)
			for _, p := range sc.Cast {
				sceneProof := proof
				if external && c.Episode == 2 {
					sceneProof = "chase-account"
				}
				b.Queue(queries.InsertSceneAppearance, v, c.Slug, sc.Slug, p, sceneProof)
			}
		}
	}
	for i, line := range w.Lines {
		b.Queue(queries.InsertStoryline, v, line.Slug, line.Title, line.Intro, i)
		for j, c := range line.Chapters {
			b.Queue(queries.InsertStorylineChapter, v, line.Slug, c, j)
		}
	}
	return tx.SendBatch(ctx, b).Close()
}
