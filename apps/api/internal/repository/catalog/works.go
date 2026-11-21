package catalog

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"anime-agent/api/internal/repository/database"
	queries "anime-agent/database/queries/catalog"
)

func Works(ctx context.Context, db *pgxpool.Pool, q, after string, limit int) ([]byte, error) {
	return database.ReadJSON(ctx, db, queries.Works, q, after, limit)
}

func Work(ctx context.Context, db *pgxpool.Pool, id string) ([]byte, error) {
	return database.ReadJSON(ctx, db, queries.Work, id)
}

func ResearchRuns(ctx context.Context, db *pgxpool.Pool, limit int) ([]byte, error) {
	return database.ReadJSON(ctx, db, queries.ResearchRuns, limit)
}

func WorldStatuses(ctx context.Context, db *pgxpool.Pool, limit int) ([]byte, error) {
	return database.ReadJSON(ctx, db, queries.WorldStatuses, limit)
}

func WorkStatus(ctx context.Context, db *pgxpool.Pool, id string) ([]byte, error) {
	return database.ReadJSON(ctx, db, queries.WorkStatus, id)
}

func Sources(ctx context.Context, db *pgxpool.Pool) ([]byte, error) {
	return database.ReadJSON(ctx, db, queries.Sources)
}

func QualityRuns(ctx context.Context, db *pgxpool.Pool, limit int) ([]byte, error) {
	return database.ReadJSON(ctx, db, queries.QualityRuns, limit)
}

func SyncRuns(ctx context.Context, db *pgxpool.Pool, source string, limit int) ([]byte, error) {
	return database.ReadJSON(ctx, db, queries.SyncRuns, source, limit)
}

func MatchCandidates(ctx context.Context, db *pgxpool.Pool, limit int) ([]byte, error) {
	return database.ReadJSON(ctx, db, queries.MatchCandidates, limit)
}
