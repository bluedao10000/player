package database

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

func ReadJSON(ctx context.Context, db *pgxpool.Pool, sql string, args ...any) ([]byte, error) {
	var data []byte
	err := db.QueryRow(ctx, sql, args...).Scan(&data)
	return data, err
}
