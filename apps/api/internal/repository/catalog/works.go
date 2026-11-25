package catalog

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"anime-agent/api/internal/repository/database"
	queries "anime-agent/api/internal/repository/queries/catalog"
)

func Works(ctx context.Context, db *pgxpool.Pool, q, after string, limit int) ([]byte, error) {
	return database.ReadJSON(ctx, db, queries.Works, q, after, limit)
}

func Work(ctx context.Context, db *pgxpool.Pool, id string) ([]byte, error) {
	return database.ReadJSON(ctx, db, queries.Work, id)
}

func Sources(ctx context.Context, db *pgxpool.Pool) ([]byte, error) {
	return database.ReadJSON(ctx, db, queries.Sources)
}

func MatchCandidates(ctx context.Context, db *pgxpool.Pool, limit int) ([]byte, error) {
	return database.ReadJSON(ctx, db, queries.MatchCandidates, limit)
}
