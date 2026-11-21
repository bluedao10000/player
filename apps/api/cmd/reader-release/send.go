package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// A session lock spans delivery and acknowledgement. A crashed worker drops the
// lock; the next holder records the interruption and resends the same final.
func send(ctx context.Context, db *pgx.Conn) error {
	var locked bool
	if err := db.QueryRow(ctx, "SELECT pg_try_advisory_lock(82045127)").Scan(&locked); err != nil || !locked {
		return err
	}
	if _, err := db.Exec(ctx, "UPDATE reader_release_attempts SET finished_at=now(),error='发布进程中断，等待重试' WHERE finished_at IS NULL"); err != nil {
		return err
	}
	var topic, attempt string
	var origin int64
	err := db.QueryRow(ctx, `WITH due AS (SELECT s.topic,s.id FROM visible_reader_scopes s JOIN episode_admissions a USING(topic)
 JOIN episode_plans p USING(topic) JOIN episode_policies pol ON pol.world_slug=p.world_slug
 WHERE p.spec @> '{"publishToReader":true}' AND NOT EXISTS(SELECT FROM reader_release_attempts r WHERE r.topic=s.topic
 AND r.acceptance=s.id AND (r.target_id IS NOT NULL OR r.finished_at>now()-interval '5 minutes'))
 ORDER BY pol.priority DESC,s.first_episode,s.topic LIMIT 1)
 INSERT INTO reader_release_attempts(topic,acceptance) SELECT * FROM due RETURNING id::text,topic,acceptance`).Scan(&attempt, &topic, &origin)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	id, href, runErr := deliver(ctx, db, topic, origin)
	var result, errorText any
	if runErr != nil {
		errorText = runErr.Error()
		if id > 0 {
			errorText = fmt.Sprintf("正式站已接收 %d，公开校验待重试：%v", id, runErr)
		}
	} else {
		result = id
	}
	_, err = db.Exec(ctx, "UPDATE reader_release_attempts SET finished_at=now(),error=$2,target_id=$3,href=$4 WHERE id=$1", attempt, errorText, result, href)
	if err != nil {
		return err
	}
	if runErr == nil {
		fmt.Println(topic, href)
	}
	return runErr
}

func deliver(ctx context.Context, db *pgx.Conn, topic string, origin int64) (int64, string, error) {
	var e envelope
	if err := db.QueryRow(ctx, exportSQL, topic, origin).Scan(&e.Payload); err != nil {
		return 0, "", err
	}
	a, sha, err := e.inspect()
	if err != nil {
		return 0, "", err
	}
	p := a.Picks[0]
	href := "/work/" + a.World + "/chapter/" + p.Chapter
	e.Image, err = os.ReadFile(filepath.Join(os.Getenv("KNOWLEDGE_ARCHIVE_DIR"), sha))
	if err != nil {
		return 0, href, err
	}
	if digest(e.Image) != sha {
		return 0, href, errors.New("source archive checksum mismatch")
	}
	payload, err := json.Marshal(e)
	if err != nil {
		return 0, href, err
	}
	if len(payload) > limit {
		return 0, href, errors.New("release exceeds transport limit")
	}
	cmd := exec.CommandContext(ctx, "ssh", "-T", "-F", os.Getenv("READER_SSH_CONFIG"), "-oBatchMode=yes", "-oConnectTimeout=10", "folio-reader")
	cmd.Stdin = bytes.NewReader(payload)
	out, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return 0, href, fmt.Errorf("receiver: %s", strings.TrimSpace(string(ee.Stderr)))
		}
		return 0, href, err
	}
	id, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	if err != nil || id < 1 {
		return 0, href, errors.New("invalid release receipt")
	}
	return id, href, verifyPublic(ctx, a, sha)
}
func verifyPublic(ctx context.Context, a acceptance, sha string) error {
	base := strings.TrimRight(os.Getenv("READER_PUBLIC_URL"), "/")
	client := http.Client{Timeout: 20 * time.Second}
	get := func(path string) ([]byte, error) {
		req, err := http.NewRequestWithContext(ctx, "GET", base+path, nil)
		if err != nil {
			return nil, err
		}
		res, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer res.Body.Close()
		if res.StatusCode != 200 {
			return nil, fmt.Errorf("%s HTTP %d", path, res.StatusCode)
		}
		return io.ReadAll(io.LimitReader(res.Body, limit))
	}
	p := a.Picks[0]
	work := "/work/" + a.World
	chapter := work + "/chapter/" + p.Chapter
	api := "/ios-data/v1/works/" + a.World
	h, err := get(chapter)
	if err != nil {
		return err
	}
	anchor := "passage-" + p.Anchor
	if !bytes.Contains(h, []byte(`id="`+anchor+`"`)) {
		return errors.New("public chapter lacks anchor")
	}
	h, err = get(work + "/scene/" + p.Anchor)
	if err != nil {
		return err
	}
	if !bytes.Contains(h, []byte(`href="`+chapter+"#"+anchor+`"`)) {
		return errors.New("public scene lacks return link")
	}
	data, err := get(api + "/scenes/" + p.Anchor)
	if err != nil {
		return err
	}
	var scene struct {
		Anchor  string
		Chapter struct{ Href string }
	}
	if json.Unmarshal(data, &scene) != nil || scene.Anchor != anchor || scene.Chapter.Href != chapter {
		return errors.New("iOS scene target mismatch")
	}
	data, err = get(api + "/chapters/" + p.Chapter)
	if err != nil {
		return err
	}
	var ch struct {
		Blocks []struct {
			Text, Anchor string
			Anchors      []string
		}
		Illustrations []struct{ Image string }
	}
	if json.Unmarshal(data, &ch) != nil || len(ch.Blocks) != len(a.Copy[p.Chapter]) || len(ch.Illustrations) != 1 || ch.Illustrations[0].Image != p.Image {
		return errors.New("iOS final differs")
	}
	found := false
	for i, b := range ch.Blocks {
		if b.Text != a.Copy[p.Chapter][i].Text {
			return errors.New("iOS text differs")
		}
		if b.Anchor == anchor {
			found = true
		}
		for _, v := range b.Anchors {
			if v == anchor {
				found = true
			}
		}
	}
	if !found {
		return errors.New("iOS chapter lacks return anchor")
	}
	image, err := get(p.Image)
	if err != nil {
		return err
	}
	if digest(image) != sha {
		return errors.New("public image checksum differs")
	}
	return nil
}
