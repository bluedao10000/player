package workbench

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"anime-agent/api/internal/repository/database"
	queries "anime-agent/database/queries/workbench"
)

func Versions(ctx context.Context, db *pgxpool.Pool, source string) ([]byte, error) {
	return database.ReadJSON(ctx, db, queries.Versions, source)
}

func Matches(ctx context.Context, db *pgxpool.Pool, limit int) ([]byte, error) {
	return database.ReadJSON(ctx, db, queries.Matches, limit)
}

func Match(ctx context.Context, db *pgxpool.Pool, id string) ([]byte, error) {
	return database.ReadJSON(ctx, db, queries.Match, id)
}

func Issues(ctx context.Context, db *pgxpool.Pool, id string) ([]byte, error) {
	return database.ReadJSON(ctx, db, queries.Issues, id)
}

func History(ctx context.Context, db *pgxpool.Pool, id string) ([]byte, error) {
	return database.ReadJSON(ctx, db, queries.History, id)
}
