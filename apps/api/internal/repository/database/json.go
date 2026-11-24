package database

import (
	"context"
	_ "embed"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	resources "anime-agent/api/internal/repository/queries"
)

//go:embed json.queries
var jsonSource string

var jsonQueries = resources.Statements(jsonSource)

type queryRower interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func ReadJSON(ctx context.Context, db queryRower, sql string, args ...any) ([]byte, error) {
	var data []byte
	err := db.QueryRow(ctx, sql, args...).Scan(&data)
	return data, err
}

// ReadJSONWithoutJIT confines the setting to a single read-only transaction.
func ReadJSONWithoutJIT(ctx context.Context, db *pgxpool.Pool, sql string, args ...any) ([]byte, error) {
	var data []byte
	err := pgx.BeginTxFunc(ctx, db, pgx.TxOptions{AccessMode: pgx.ReadOnly}, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, jsonQueries["DisableJIT"]); err != nil {
			return err
		}

		// Plan after SET LOCAL, even if the same SQL was prepared with JIT enabled.
		queryArgs := append([]any{pgx.QueryExecModeExec}, args...)
		var err error
		data, err = ReadJSON(ctx, tx, sql, queryArgs...)
		return err
	})
	return data, err
}
