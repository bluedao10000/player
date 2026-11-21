package reader

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	model "anime-agent/api/internal/model/reader"
	queries "anime-agent/api/internal/repository/queries/reader"
)

func SavePosition(ctx context.Context, tx pgx.Tx, id string, e model.Entry) error {
	_, err := tx.Exec(ctx, queries.SavePosition, id, e.Href, e.Title, e.Work, e.Percent, e.Anchor, e.Excerpt, min(e.RecordedAt, time.Now().UnixMilli()))
	return err
}

func SaveBookmark(ctx context.Context, tx pgx.Tx, id string, e model.Entry) error {
	_, err := tx.Exec(ctx, queries.SaveBookmark, id, e.Href, e.Title, e.Work, min(e.RecordedAt, time.Now().UnixMilli()))
	return err
}

func State(ctx context.Context, db *pgxpool.Pool, id, mobileApp, webApp string) (json.RawMessage, error) {
	var result json.RawMessage
	err := db.QueryRow(ctx, queries.State, id, mobileApp, webApp).Scan(&result)
	return result, err
}
