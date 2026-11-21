package knowledge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	queries "anime-agent/api/internal/repository/queries/knowledge"
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
		_, err := tx.Exec(ctx, queries.FinishMediaStep, j.Slot, j.Round, j.Step, reply.Result, reply.Error, j.Attempt)
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
			tag, err := tx.Exec(ctx, queries.PublishPortrait, j.Slot, j.Step, j.Attempt)
			if err != nil {
				return err
			}
			if tag.RowsAffected() != 1 {
				return fmt.Errorf("portrait slot %d missing matching render and review receipts", j.Slot)
			}
		}
		_, err = tx.Exec(ctx, queries.AdvanceCandidate, j.Slot, next, round, focus, reason, attempt, j.ReferenceSHA)
		return err
	})
}

// ResumeImageProviderFailures retries generation only when an earlier provider
// request failed. Finished images and quality decisions keep their identities.
func ResumeImageProviderFailures(ctx context.Context, db *pgxpool.Pool, world string) (int64, error) {
	if !planSlug.MatchString(world) {
		return 0, fmt.Errorf("无效作品标识")
	}
	result, err := db.Exec(ctx, queries.RequeueProviderFailures, world)
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
	if err = pc.QueryRow(ctx, queries.TryLockMedia).Scan(&locked); err != nil || !locked {
		return err
	}
	defer pc.Exec(context.Background(), queries.UnlockMedia)
	rows, err := pc.Query(ctx, queries.NextCandidate)
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
			if _, err = pc.Exec(ctx, queries.StartGeneration, j.Slot); err != nil {
				return err
			}
		}
		if _, err = pc.Exec(ctx, queries.StartMediaStep, j.Slot, j.Round, j.Step, j.Attempt); err != nil {
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
