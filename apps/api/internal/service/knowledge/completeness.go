package knowledge

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	queries "anime-agent/api/internal/repository/queries/knowledge"
)

// RecalculateCompleteness stores a repeatable result for each currently
// published world version. Results for older published versions are retained.
func RecalculateCompleteness(ctx context.Context, db *pgxpool.Pool) error {
	return pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, queries.CalculateCompleteness)
		return err
	})
}
