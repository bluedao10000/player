package catalog

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"anime-agent/api/internal/repository/database"
	queries "anime-agent/api/internal/repository/queries/catalog"
)

func Discovery(ctx context.Context, db *pgxpool.Pool) ([]byte, error) {
	return database.ReadJSON(ctx, db, queries.Discovery)
}

func HomeWorlds(ctx context.Context, db *pgxpool.Pool) ([]byte, error) {
	return database.ReadJSON(ctx, db, queries.HomeWorlds)
}

func World(ctx context.Context, db *pgxpool.Pool, work string) ([]byte, error) {
	return database.ReadJSON(ctx, db, queries.World, work)
}

func Character(ctx context.Context, db *pgxpool.Pool, work, person string) ([]byte, error) {
	return database.ReadJSON(ctx, db, queries.Character, work, person)
}

func Chapter(ctx context.Context, db *pgxpool.Pool, work, chapter string) ([]byte, error) {
	return database.ReadJSON(ctx, db, queries.Chapter, work, chapter)
}

func Scene(ctx context.Context, db *pgxpool.Pool, work, scene string) ([]byte, error) {
	return database.ReadJSON(ctx, db, queries.Scene, work, scene)
}
