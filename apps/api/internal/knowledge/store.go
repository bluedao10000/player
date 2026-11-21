package knowledge

import (
	"context"
	"embed"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed sql/*.sql
var migrations embed.FS

func Open(ctx context.Context) (*pgxpool.Pool, error) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		return nil, fmt.Errorf("DATABASE_URL is required (PostgreSQL)")
	}
	db, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}
	// Remote production workers may read the migration ledger without owning DDL.
	if os.Getenv("KNOWLEDGE_VERIFY_SCHEMA_ONLY") == "true" {
		files, e := migrations.ReadDir("sql")
		if e != nil {
			db.Close()
			return nil, e
		}
		expected := make([]string, 0, len(files))
		for _, file := range files {
			expected = append(expected, file.Name())
		}
		var installed int
		e = db.QueryRow(ctx, "SELECT count(*) FROM knowledge_migrations WHERE version=ANY($1::text[])", expected).Scan(&installed)
		if e == nil && installed != len(expected) {
			e = fmt.Errorf("schema requires %d migrations, found %d", len(expected), installed)
		}
		if e != nil {
			db.Close()
			return nil, e
		}
		return db, nil
	}
	err = pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(82045101)"); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, "CREATE TABLE IF NOT EXISTS knowledge_migrations(version text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now())"); err != nil {
			return err
		}
		files, err := migrations.ReadDir("sql")
		if err != nil {
			return err
		}
		for _, file := range files {
			var applied bool
			if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT FROM knowledge_migrations WHERE version=$1)", file.Name()).Scan(&applied); err != nil {
				return err
			}
			if applied {
				continue
			}
			body, err := migrations.ReadFile("sql/" + file.Name())
			if err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, string(body)); err != nil {
				return fmt.Errorf("migration %s: %w", file.Name(), err)
			}
			if _, err = tx.Exec(ctx, "INSERT INTO knowledge_migrations(version) VALUES($1)", file.Name()); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}
