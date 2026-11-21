package workbench

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"anime-agent/api/internal/repository/database"
	queries "anime-agent/api/internal/repository/queries/workbench"
)

func MediaCandidates(ctx context.Context, db *pgxpool.Pool) ([]byte, error) {
	return database.ReadJSON(ctx, db, queries.MediaCandidates)
}
