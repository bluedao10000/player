package knowledge

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	queries "anime-agent/api/internal/repository/queries/knowledge"
)

func Quality(ctx context.Context, db *pgxpool.Pool, source, work string) (json.RawMessage, error) {
	var report json.RawMessage
	err := pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, queries.LockProjection); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, queries.CreateQualityScope); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, queries.FillQualityScope, source, work)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return fmt.Errorf("no active works in scope")
		}
		if _, err = tx.Exec(ctx, queries.Quality); err != nil {
			return err
		}
		return tx.QueryRow(ctx, queries.QualityReport).Scan(&report)
	})
	return report, err
}
