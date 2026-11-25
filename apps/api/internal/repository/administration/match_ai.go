package administration

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"anime-agent/api/internal/repository/database"
	queries "anime-agent/api/internal/repository/queries/administration"
)

func Assessments(ctx context.Context, db *pgxpool.Pool, id string) ([]byte, error) {
	return database.ReadJSON(ctx, db, queries.Assessments, id)
}
