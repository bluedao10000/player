package testsupport

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"anime-agent/api/internal/repository/database"
	sqltest "anime-agent/api/internal/testsupport/fixtures"
)

func Database(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("KNOWLEDGE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set KNOWLEDGE_TEST_DATABASE_URL for PostgreSQL integration")
	}
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { admin.Close(ctx) })
	ns := fmt.Sprintf("kb_test_%d", time.Now().UnixNano())
	if _, err = admin.Exec(ctx, fmt.Sprintf(sqltest.CreateSchema, pgx.Identifier{ns}.Sanitize())); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { admin.Exec(ctx, fmt.Sprintf(sqltest.DropSchema, pgx.Identifier{ns}.Sanitize())) })
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	q.Set("search_path", ns)
	u.RawQuery = q.Encode()
	t.Setenv("DATABASE_URL", u.String())
	db, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	return db
}
