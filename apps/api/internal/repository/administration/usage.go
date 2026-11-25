package administration

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"anime-agent/api/internal/repository/database"
	queries "anime-agent/api/internal/repository/queries/administration"
)

func ModelUsage(ctx context.Context, db *pgxpool.Pool, from, to time.Time) ([]byte, error) {
	return database.ReadJSON(ctx, db, queries.ModelUsage, from, to)
}

func Calls(ctx context.Context, db *pgxpool.Pool, from, to time.Time, before int64, subject string, limit int) ([]byte, error) {
	return database.ReadJSON(ctx, db, queries.Calls, from, to, before, subject, limit)
}
