package knowledge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type mediaJob struct {
	Art          json.RawMessage `json:"art,omitempty"`
	Batch        string          `json:"batch"`
	Position     int             `json:"position"`
	ReferenceSHA string          `json:"referenceSHA"`
	Attempt      int             `json:"attempt"`
	Slot         int             `json:"slot"`
	Round        int             `json:"round"`
	Step         string          `json:"step"`
	Focus        int             `json:"focus"`
	Reason       string          `json:"reason"`
}
type mediaReply struct {
	State  string
	Result json.RawMessage
	Error  string
}

func pollMedia(ctx context.Context, base string, j mediaJob) (mediaReply, error) {
	var out mediaReply
	payload := struct {
		mediaJob
		ReviewVersion int `json:"reviewVersion,omitempty"`
	}{mediaJob: j}
	if j.Step == "review" || j.Step == "recheck" {
		payload.ReviewVersion = 2
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(base, "/")+"/internal/media", bytes.NewReader(body))
	if err != nil {
		return out, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 12 * time.Second}).Do(req)
	if err != nil {
		return out, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return out, fmt.Errorf("Pi media HTTP %d", resp.StatusCode)
	}
	err = json.NewDecoder(io.LimitReader(resp.Body, 32000)).Decode(&out)
	return out, err
}

func advanceMedia(ctx context.Context, conn *pgx.Conn, j mediaJob, reply mediaReply) error {
	var r struct {
		Decision, Reason, Image, Phone, SHA, Reference_SHA string
		Focus                                              int
	}
	next, round, focus, reason := "", j.Round, j.Focus, j.Reason
	attempt := 1
	switch reply.State {
	case "running":
		return nil
	case "failed":
		next = "failed"
		reason = reply.Error
		// Retry transient provider/service errors once, preserving the original and the audit record.
		if j.Attempt == 1 && (strings.Contains(reason, "HTTP 429") || strings.Contains(reason, "HTTP 5") || strings.HasPrefix(reason, "EACCES:")) {
			next = j.Step
			attempt = 2
			reason = j.Reason
		}
	case "done":
		if err := json.Unmarshal(reply.Result, &r); err != nil {
			return err
		}
		switch j.Step {
		case "reference":
			if len(r.Reference_SHA) != 64 {
				return fmt.Errorf("missing reference fingerprint")
			}
			next = "generate"
			j.ReferenceSHA = r.Reference_SHA
		case "generate":
			if r.Image == "" || r.Phone == "" || len(r.SHA) != 64 || len(r.Reference_SHA) != 64 || (j.ReferenceSHA != "" && j.ReferenceSHA != r.Reference_SHA) {
				return fmt.Errorf("missing generated image manifest")
			}
			next = "review"
		case "recrop":
			if r.Phone == "" {
				return fmt.Errorf("missing revised crop")
			}
			next = "recheck"
		case "review", "recheck":
			reason = r.Reason
			switch {
			case r.Decision == "pass":
				next = "passed"
			case r.Decision == "recrop" && j.Step == "review" && r.Focus != j.Focus && r.Focus >= 0 && r.Focus <= 100:
				next = "recrop"
				focus = r.Focus
			case (r.Decision == "redraw" || r.Decision == "recrop") && j.Round == 1:
				next = "generate"
				round++
				focus = 40
			case r.Decision == "redraw" || r.Decision == "recrop":
				next = "held"
			default:
				return fmt.Errorf("unknown visual assessment")
			}
		}
	default:
		return fmt.Errorf("unknown task state")
	}
	return pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE media_steps SET finished_at=now(),result=$4,error=nullif($5,'') WHERE slot=$1 AND round=$2 AND step=$3 AND attempt=$6`, j.Slot, j.Round, j.Step, reply.Result, reply.Error, j.Attempt)
		if err != nil {
			return err
		}
		var art struct{ Kind string }
		if len(j.Art) > 0 {
			if err := json.Unmarshal(j.Art, &art); err != nil {
				return err
			}
		}
		if next == "passed" && (j.Batch == "bebop-portrait" || art.Kind == "portrait") {
			// Publish the derivative for this reviewed round in the same commit as
			// the pass. The source PNG and rejected rounds stay workbench-only.
			tag, err := tx.Exec(ctx, `INSERT INTO edition_reader_art(world_slug,kind,item_slug,image,caption,credit,source_sha256)
				SELECT coalesce(a.world_slug,'cowboy-bebop-tv'),'character',coalesce(a.slug,c.character_slug),
				       '/art/portraits/'||(g.result->>'public_sha')||'.webp',c.scene,'AI 人物肖像',g.result->>'sha'
				FROM media_candidates c LEFT JOIN episode_art_jobs a ON a.id=c.art_job JOIN media_steps g ON g.slot=c.slot AND g.round=c.round AND g.step='generate'
				JOIN media_steps r ON r.slot=c.slot AND r.round=c.round AND r.step=$2 AND r.attempt=$3
				WHERE c.slot=$1 AND (a.kind='portrait' OR c.batch='bebop-portrait') AND g.error IS NULL AND g.finished_at IS NOT NULL
				  AND g.result->>'public_sha' ~ '^[0-9a-f]{64}$' AND g.result->>'reference_sha'=c.reference_sha
				  AND r.result->>'decision'='pass' AND r.result->>'phone'=g.result->>'phone'
				  AND r.result->'checks' @> '{"identity":true,"scene":true,"anatomy":true,"clean":true,"crop":true}'
				ON CONFLICT (world_slug,kind,item_slug) DO UPDATE SET image=excluded.image,caption=excluded.caption,
				    credit=excluded.credit,source_sha256=excluded.source_sha256`, j.Slot, j.Step, j.Attempt)
			if err != nil {
				return err
			}
			if tag.RowsAffected() != 1 {
				return fmt.Errorf("portrait slot %d missing matching render and review receipts", j.Slot)
			}
		}
		_, err = tx.Exec(ctx, `UPDATE media_candidates SET stage=$2,round=$3,focus=$4,reason=$5,attempt=$6,reference_sha=$7,updated_at=now() WHERE slot=$1`, j.Slot, next, round, focus, reason, attempt, j.ReferenceSHA)
		return err
	})
}

// ResumeImageProviderFailures retries generation only when an earlier provider
// request failed. Finished images and quality decisions keep their identities.
func ResumeImageProviderFailures(ctx context.Context, db *pgxpool.Pool, world string) (int64, error) {
	if !planSlug.MatchString(world) {
		return 0, fmt.Errorf("无效作品标识")
	}
	result, err := db.Exec(ctx, `UPDATE media_candidates c SET stage='generate',
	    attempt=(SELECT max(s.attempt)+1 FROM media_steps s WHERE s.slot=c.slot AND s.round=c.round AND s.step='generate'),
	    reason=coalesce((SELECT s.result->>'reason' FROM media_steps s WHERE s.slot=c.slot AND s.round=c.round-1
	        AND s.step IN ('review','recheck') AND s.result->>'decision' IN ('redraw','recrop')
	        ORDER BY s.finished_at DESC LIMIT 1),''),updated_at=now()
	  FROM episode_art_jobs a WHERE c.art_job=a.id AND a.world_slug=$1 AND c.stage='failed'
	    AND c.reason ~ '^模型服务 HTTP (401|403|429|5[0-9]{2})$'
	    AND EXISTS(SELECT FROM media_steps s WHERE s.slot=c.slot AND s.round=c.round AND s.step='generate' AND s.error=c.reason)
	    AND NOT EXISTS(SELECT FROM media_steps s WHERE s.slot=c.slot AND s.round=c.round AND s.step='generate' AND s.error IS NULL)`, world)
	return result.RowsAffected(), err
}

// The advisory lock serializes admission across worker replicas. Polling reuses
// the Pi task identity even if the worker exits before committing its result.
func SyncMedia(ctx context.Context, db *pgxpool.Pool, base string) error {
	pc, err := db.Acquire(ctx)
	if err != nil {
		return err
	}
	defer pc.Release()
	var locked bool
	if err = pc.QueryRow(ctx, "SELECT pg_try_advisory_lock(82045120)").Scan(&locked); err != nil || !locked {
		return err
	}
	defer pc.Exec(context.Background(), "SELECT pg_advisory_unlock(82045120)")
	rows, err := pc.Query(ctx, `SELECT (SELECT spec FROM episode_art_jobs WHERE id=art_job),batch,position,reference_sha,attempt,slot,round,stage,focus,reason FROM media_candidates WHERE stage NOT IN ('passed','held','failed') ORDER BY (SELECT priority FROM episode_policies pol JOIN episode_art_jobs art ON art.world_slug=pol.world_slug WHERE art.id=media_candidates.art_job) DESC NULLS LAST,(position-1)%3,slot LIMIT 3`)
	if err != nil {
		return err
	}
	jobs, err := pgx.CollectRows(rows, pgx.RowToStructByPos[mediaJob])
	if err != nil {
		return err
	}
	for _, j := range jobs {
		if j.Step == "queued" {
			j.Step = "generate"
			if _, err = pc.Exec(ctx, "UPDATE media_candidates SET stage='generate',updated_at=now() WHERE slot=$1", j.Slot); err != nil {
				return err
			}
		}
		if _, err = pc.Exec(ctx, "INSERT INTO media_steps(slot,round,step,attempt) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING", j.Slot, j.Round, j.Step, j.Attempt); err != nil {
			return err
		}
		reply, e := pollMedia(ctx, base, j)
		if e != nil {
			return e
		}
		if e = advanceMedia(ctx, pc.Conn(), j, reply); e != nil {
			return e
		}
	}
	return nil
}

var assetName = regexp.MustCompile(`^((mio|bebop|bebop-portrait|bebop-e3|cfg-[1-9][0-9]*)-[1-9]-r[12](\.png|-crop[01]\.jpg)|bebop-reference-v1\.png)$`)

var publicPortraitName = regexp.MustCompile(`^[a-f0-9]{64}\.webp$`)

func mediaRoutes(g *gin.RouterGroup, db *pgxpool.Pool) {
	mediaDir := os.Getenv("KNOWLEDGE_MEDIA_DIR")
	if mediaDir == "" {
		mediaDir = "/media"
	}
	g.GET("/media-candidates", func(c *gin.Context) {
		queryJSON(c, db, `SELECT jsonb_agg(to_jsonb(x) ORDER BY x.slot) FROM (
    SELECT c.*,a.spec AS art,CASE WHEN c.batch='bebop-portrait' AND c.stage='passed' THEN
       (SELECT card->>'href' FROM published_characters WHERE slug=c.character_slug AND v=(SELECT version_id FROM work_worlds WHERE slug='cowboy-bebop-tv')
        UNION ALL SELECT card->>'href' FROM extension_people WHERE slug=c.character_slug AND world_slug='cowboy-bebop-tv' LIMIT 1) END AS reader_href,
       coalesce((SELECT jsonb_agg(to_jsonb(s)-'slot' ORDER BY started_at) FROM media_steps s WHERE s.slot=c.slot),'[]') AS steps
    FROM media_candidates c LEFT JOIN episode_art_jobs a ON a.id=c.art_job)x`)
	})
	g.GET("/reader-portraits/:file", func(c *gin.Context) {
		name := c.Param("file")
		if !publicPortraitName.MatchString(name) {
			c.Status(404)
			return
		}
		var published bool
		err := db.QueryRow(c.Request.Context(), `SELECT EXISTS(SELECT FROM edition_reader_art WHERE kind='character' AND image=$1)`, "/art/portraits/"+name).Scan(&published)
		if err != nil {
			c.Status(503)
			return
		}
		if !published {
			c.Status(404)
			return
		}
		c.Header("Cache-Control", "public, max-age=31536000, immutable")
		c.File(filepath.Join(mediaDir, name))
	})
	g.GET("/media-assets/:file", func(c *gin.Context) {
		name := c.Param("file")
		if !assetName.MatchString(name) {
			c.Status(404)
			return
		}
		path := filepath.Join(mediaDir, name)
		if _, err := os.Stat(path); err != nil {
			c.Status(404)
			return
		}
		c.Header("Cache-Control", "private, max-age=86400, immutable")
		c.Header("X-Robots-Tag", "noindex")
		c.File(path)
	})
}
