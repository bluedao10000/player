package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"anime-agent/api/internal/config"
	"anime-agent/database/migrations"
)

// Open shares the existing pgx pool with GORM during schema initialization.
// Workers using a restricted production login only read the migration ledger.
func Open(ctx context.Context) (*pgxpool.Pool, error) {
	url, verifyOnly := config.Database()
	if url == "" {
		return nil, fmt.Errorf("DATABASE_URL is required (PostgreSQL)")
	}
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}
	if err = ensureSchema(ctx, pool, verifyOnly); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}

func ensureSchema(ctx context.Context, pool *pgxpool.Pool, verifyOnly bool) error {
	adapter := stdlib.OpenDBFromPool(pool)
	defer adapter.Close()
	orm, err := gorm.Open(postgres.New(postgres.Config{Conn: adapter}), &gorm.Config{DisableAutomaticPing: true, Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return err
	}
	db := orm.WithContext(ctx)
	if verifyOnly {
		return migrations.Verify(db)
	}
	return migrations.Apply(db)
}
