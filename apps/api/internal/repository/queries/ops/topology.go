package ops

import (
	"context"
	_ "embed"
	"time"

	resources "anime-agent/api/internal/repository/queries"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed topology.queries
var topologySource string

var topologyQueries = resources.Statements(topologySource)

func TopologyWorldExists(ctx context.Context, db *pgxpool.Pool, world string) (bool, error) {
	var exists bool
	err := db.QueryRow(ctx, topologyQueries["TopologyWorldExists"], world).Scan(&exists)
	return exists, err
}

func Topology(ctx context.Context, db *pgxpool.Pool, window string, hours int, world string) ([]byte, error) {
	return TopologyAt(ctx, db, window, hours, world, nil)
}

// TopologyAt accepts a fixed clock for deterministic query-boundary tests.
// Production callers use Topology, which relies on statement_timestamp().
func TopologyAt(ctx context.Context, db *pgxpool.Pool, window string, hours int, world string, at *time.Time) ([]byte, error) {
	var target any
	if world != "" {
		target = world
	}
	var clock any
	if at != nil {
		clock = *at
	}
	var body []byte
	err := db.QueryRow(ctx, topologyQueries["Topology"], window, hours, target, clock).Scan(&body)
	return body, err
}

func TopologyStageDetail(ctx context.Context, db *pgxpool.Pool, stage string, hours int, world string,
	queueAt *string, queueKey string, runAt *string, runKey string, limit int) ([]byte, error) {
	var target any
	if world != "" {
		target = world
	}
	var body []byte
	err := db.QueryRow(ctx, topologyQueries["TopologyStageDetail"],
		stage, hours, target, queueAt, queueKey, runAt, runKey, limit,
	).Scan(&body)
	return body, err
}
