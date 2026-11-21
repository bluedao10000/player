package knowledge

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The external collector uses the same parser as the single-node Pi tool.
// Match the registered edition and both source URLs before a capture can be used.
func ImportEpisodeCapture(ctx context.Context, db *pgxpool.Pool, input io.Reader) error {
	var capture struct {
		Lease, Error string
		World        string
		Spec         json.RawMessage
		Documents    []researchDocument
	}
	if err := json.NewDecoder(io.LimitReader(input, 256<<10)).Decode(&capture); err != nil {
		return err
	}
	var spec episodeSpec
	if err := json.Unmarshal(capture.Spec, &spec); err != nil {
		return err
	}
	return pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		if capture.Lease != "" {
			tag, err := tx.Exec(ctx, `UPDATE episode_capture_attempts a SET finished_at=now(),error=nullif(left($4,1000),'') FROM episode_plans p
   WHERE a.id=$1 AND a.topic=p.topic AND p.world_slug=$2 AND p.spec=$3 AND a.finished_at IS NULL`, capture.Lease, capture.World, capture.Spec, capture.Error)
			if err != nil {
				return err
			}
			if tag.RowsAffected() != 1 {
				return errors.New("采集回执与有效租约不符")
			}
			if capture.Error != "" {
				return nil
			}
		} else if capture.Error != "" {
			return errors.New("无租约的失败回执")
		}
		if err := validateSourceDocuments(capture.Documents, spec.Number, spec.Number, &spec); err != nil {
			return err
		}
		oldest := time.Now()
		for _, doc := range capture.Documents {
			if doc.Fetched.Before(time.Now().Add(-24*time.Hour)) || doc.Fetched.After(time.Now().Add(time.Minute)) {
				return errors.New("来源快照已过期或时间异常")
			}
			if doc.Fetched.Before(oldest) {
				oldest = doc.Fetched
			}
		}
		result, err := tx.Exec(ctx, `INSERT INTO episode_source_captures(topic,spec,documents,captured_at)
	SELECT topic,spec,$3,$4 FROM episode_plans WHERE world_slug=$1 AND spec=$2::jsonb
	ON CONFLICT(topic) DO UPDATE SET spec=excluded.spec,documents=excluded.documents,captured_at=excluded.captured_at
	WHERE episode_source_captures.captured_at<=excluded.captured_at`, capture.World, capture.Spec, capture.Documents, oldest)
		if err == nil && result.RowsAffected() != 1 {
			return errors.New("快照未匹配当前选题或新版快照已存在")
		}
		return err
	})
}
