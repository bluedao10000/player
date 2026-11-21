package workbench

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"anime-agent/api/internal/repository/database"
	queries "anime-agent/database/queries/workbench"
)

func BackupStatus(ctx context.Context, db *pgxpool.Pool) ([]byte, error) {
	return database.ReadJSON(ctx, db, queries.BackupStatus)
}

func EpisodeQueue(ctx context.Context, db *pgxpool.Pool) ([]byte, error) {
	return database.ReadJSON(ctx, db, queries.EpisodeQueue)
}

func ReaderReleases(ctx context.Context, db *pgxpool.Pool) ([]byte, error) {
	return database.ReadJSON(ctx, db, queries.ReaderReleases)
}

func EpisodeInventory(ctx context.Context, db *pgxpool.Pool) ([]byte, error) {
	return database.ReadJSON(ctx, db, queries.EpisodeInventory)
}

func ProductionUsage(ctx context.Context, db *pgxpool.Pool, limit int) ([]byte, error) {
	return database.ReadJSON(ctx, db, queries.ProductionUsage, limit)
}

func ModelUsage(ctx context.Context, db *pgxpool.Pool, from, to time.Time) ([]byte, error) {
	return database.ReadJSON(ctx, db, queries.ModelUsage, from, to)
}

func Calls(ctx context.Context, db *pgxpool.Pool, from, to time.Time, before int64, subject string, limit int) ([]byte, error) {
	return database.ReadJSON(ctx, db, queries.Calls, from, to, before, subject, limit)
}
