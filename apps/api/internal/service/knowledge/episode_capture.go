package knowledge

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	queries "anime-agent/database/queries/knowledge"
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
			tag, err := tx.Exec(ctx, queries.FinishCaptureLease, capture.Lease, capture.World, capture.Spec, capture.Error)
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
		result, err := tx.Exec(ctx, queries.SaveCapture, capture.World, capture.Spec, capture.Documents, oldest)
		if err == nil && result.RowsAffected() != 1 {
			return errors.New("快照未匹配当前选题或新版快照已存在")
		}
		return err
	})
}
