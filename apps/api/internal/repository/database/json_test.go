package database_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"anime-agent/api/internal/repository/database"
	"anime-agent/api/internal/testsupport"
)

func TestReadJSONWithoutJITRestoresConnection(t *testing.T) {
	ctx := context.Background()
	config := testsupport.Database(t).Config()
	config.MaxConns = 1
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	checkDefault := func() uint32 {
		t.Helper()
		conn, err := pool.Acquire(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Release()

		var jit string
		if err := conn.QueryRow(ctx, "SHOW jit").Scan(&jit); err != nil || jit != "on" {
			t.Fatalf("ordinary query jit=%q, err=%v, want on", jit, err)
		}
		return conn.Conn().PgConn().PID()
	}
	pid := checkDefault()
	for statement, want := range map[string]string{"SHOW jit": "off", "SHOW transaction_read_only": "on"} {
		value, err := database.ReadJSONWithoutJIT(ctx, pool, statement)
		if err != nil || string(value) != want {
			t.Fatalf("%s in transaction returned %q, err=%v, want %q", statement, value, err, want)
		}
		if got := checkDefault(); got != pid {
			t.Fatalf("connection replaced after commit: pid=%d, want %d", got, pid)
		}
	}
	_, err = database.ReadJSONWithoutJIT(ctx, pool, "SHOW nonexistent_round2_setting")
	var postgresError *pgconn.PgError
	if !errors.As(err, &postgresError) || postgresError.Code != "42704" {
		t.Fatalf("expected original PostgreSQL query error, got %v", err)
	}
	if got := checkDefault(); got != pid {
		t.Fatalf("connection replaced after rollback: pid=%d, want %d", got, pid)
	}
	t.Logf("same backend %d: JIT off in read-only transaction, JIT on after commit and rollback", pid)
}
