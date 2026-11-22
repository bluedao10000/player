package ops

import (
	"context"
	_ "embed"
	"os"
	"strings"
	"time"

	resources "anime-agent/api/internal/repository/queries"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed ops.queries
var opsSource string

var opsQueries = resources.Statements(opsSource)

type Heartbeat struct {
	WorkerID        string
	Loop            string
	Role            string
	Node            string
	Version         string
	ExpectedSeconds int
	Executions      []byte
}

func Overview(ctx context.Context, db *pgxpool.Pool) ([]byte, error) {
	var body []byte
	err := db.QueryRow(ctx, opsQueries["Overview"]).Scan(&body)
	return body, err
}

func Attention(ctx context.Context, db *pgxpool.Pool) ([]byte, error) {
	var body []byte
	err := db.QueryRow(ctx, opsQueries["Attention"]).Scan(&body)
	return body, err
}

func WriteHeartbeatSuccess(ctx context.Context, db *pgxpool.Pool, beat Heartbeat) error {
	_, err := db.Exec(ctx, opsQueries["HeartbeatSuccess"],
		beat.WorkerID, beat.Loop, beat.Role, beat.Node, beat.Version,
		beat.ExpectedSeconds, beat.Executions)
	return err
}

func WriteHeartbeatFailure(ctx context.Context, db *pgxpool.Pool, beat Heartbeat, cause string, consecutive int) error {
	_, err := db.Exec(ctx, opsQueries["HeartbeatFailure"],
		beat.WorkerID, beat.Loop, beat.Role, beat.Node, beat.Version,
		beat.ExpectedSeconds, TruncateError(cause), consecutive)
	return err
}

func WriteAgentHeartbeatFailure(ctx context.Context, db *pgxpool.Pool, beat Heartbeat, cause string, consecutive int) error {
	_, err := db.Exec(ctx, opsQueries["AgentHeartbeatFailure"],
		beat.WorkerID, beat.Loop, beat.Role, beat.Node, beat.Version,
		beat.ExpectedSeconds, TruncateError(cause), consecutive)
	return err
}

func WorkerStatusAt(ctx context.Context, db *pgxpool.Pool, workerID, loop string, at time.Time) (string, error) {
	var status string
	err := db.QueryRow(ctx, opsQueries["HeartbeatStatusAt"], workerID, loop, at).Scan(&status)
	return status, err
}

func TakeDailySnapshot(ctx context.Context, db *pgxpool.Pool, day time.Time) error {
	return pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, opsQueries["TakeDailySnapshot"], day.Format("2006-01-02"))
		return err
	})
}

func TruncateError(value string) string {
	runes := []rune(value)
	if len(runes) <= 500 {
		return value
	}
	return string(runes[:500])
}

var beijing = time.FixedZone("Asia/Shanghai", 8*60*60)

func BeijingSnapshotDay(now time.Time) (time.Time, bool) {
	local := now.In(beijing)
	due := local.Hour() > 0 || local.Hour() == 0 && local.Minute() >= 5
	day := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, beijing)
	return day, due
}

func Version() string {
	if version := strings.TrimSpace(os.Getenv("APP_VERSION")); version != "" {
		return version
	}
	return "dev"
}
