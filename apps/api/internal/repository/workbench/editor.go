package workbench

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	model "anime-agent/api/internal/model/knowledge"
	"anime-agent/api/internal/repository/database"
	queries "anime-agent/api/internal/repository/queries/workbench"
)

func Tasks(ctx context.Context, db *pgxpool.Pool, scopes []model.EditorScope) ([]byte, error) {
	return database.ReadJSON(ctx, db, queries.Tasks, scopes)
}

func Task(ctx context.Context, db *pgxpool.Pool, id string) ([]byte, error) {
	return database.ReadJSON(ctx, db, queries.Task, id)
}

func Preview(ctx context.Context, db *pgxpool.Pool, id, round, attempt string) ([]byte, error) {
	return database.ReadJSON(ctx, db, queries.Preview, id, round, attempt)
}
