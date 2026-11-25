package database

import (
	"context"

	"github.com/jackc/pgx/v5"
)

type queryRower interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func ReadJSON(ctx context.Context, db queryRower, sql string, args ...any) ([]byte, error) {
	var data []byte
	err := db.QueryRow(ctx, sql, args...).Scan(&data)
	return data, err
}
