package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	ops "anime-agent/api/internal/repository/queries/ops"
	"github.com/jackc/pgx/v5/pgxpool"
)

var validNodeName = regexp.MustCompile("^[A-Za-z0-9][A-Za-z0-9_-]{0,62}$")

func requireNodeName() (string, error) {
	node := strings.TrimSpace(os.Getenv("OPS_NODE_NAME"))
	if !validNodeName.MatchString(node) {
		return "", errors.New("OPS_NODE_NAME is required and must contain only letters, numbers, hyphens, or underscores")
	}
	return node, nil
}

type heartbeatGate struct {
	lastState         string
	lastWriteAt       time.Time
	consecutiveErrors int
}

func (g *heartbeatGate) observe(err error, at time.Time) (string, int, bool) {
	state := "success"
	if err == nil {
		g.consecutiveErrors = 0
	} else {
		state = "failed"
		g.consecutiveErrors++
	}
	write := g.lastState != state || g.lastWriteAt.IsZero() || at.Sub(g.lastWriteAt) >= 30*time.Second
	return state, g.consecutiveErrors, write
}

func (g *heartbeatGate) committed(state string, at time.Time) {
	g.lastState = state
	g.lastWriteAt = at
}

type loopReporter struct {
	db           *pgxpool.Pool
	beat         ops.Heartbeat
	agentFailure bool
	gate         heartbeatGate
}

func newLoopReporter(db *pgxpool.Pool, node, role, loop string, interval time.Duration, agentFailure bool) *loopReporter {
	return &loopReporter{
		db: db,
		beat: ops.Heartbeat{
			WorkerID:        node + "/" + role,
			Loop:            loop,
			Role:            role,
			Node:            node,
			Version:         ops.Version(),
			ExpectedSeconds: max(1, int(interval/time.Second)),
		},
		agentFailure: agentFailure,
	}
}

func (r *loopReporter) record(ctx context.Context, cause error, executions []byte) {
	if ctx.Err() != nil {
		return
	}
	now := time.Now()
	state, consecutive, write := r.gate.observe(cause, now)
	if !write {
		return
	}
	beat := r.beat
	beat.Executions = executions
	var err error
	if cause == nil {
		err = ops.WriteHeartbeatSuccess(ctx, r.db, beat)
	} else if r.agentFailure {
		err = ops.WriteAgentHeartbeatFailure(ctx, r.db, beat, cause.Error(), consecutive)
	} else {
		err = ops.WriteHeartbeatFailure(ctx, r.db, beat, cause.Error(), consecutive)
	}
	if err != nil {
		log.Printf("ops heartbeat %s/%s: %v", beat.WorkerID, beat.Loop, err)
		return
	}
	r.gate.committed(state, now)
}

func reportLoop(ctx context.Context, reporter *loopReporter, label string, err error) {
	if ctx.Err() != nil {
		return
	}
	if err != nil {
		log.Printf("%s: %v", label, err)
	}
	reporter.record(ctx, err, nil)
}

func addCycleError(ctx context.Context, result *error, label string, err error) {
	if err == nil || ctx.Err() != nil {
		return
	}
	log.Printf("%s: %v", label, err)
	*result = errors.Join(*result, err)
}

type agentStatus struct {
	Version    string         "json:\"version\""
	StartedAt  string         "json:\"startedAt\""
	Executions map[string]int "json:\"executions\""
}

func fetchAgentStatus(ctx context.Context, base string) (agentStatus, error) {
	var result agentStatus
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(base, "/")+"/internal/status", nil)
	if err != nil {
		return result, err
	}
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		return result, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return result, fmt.Errorf("Agent status HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return result, err
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return result, err
	}
	if result.Version == "" || result.StartedAt == "" || result.Executions == nil {
		return result, errors.New("Agent status response is incomplete")
	}
	for kind, count := range result.Executions {
		if count < 0 {
			return result, fmt.Errorf("Agent status has a negative %s execution count", kind)
		}
	}
	return result, nil
}

func startAgentStatusLoop(ctx context.Context, db *pgxpool.Pool, node, base string, jobs *sync.WaitGroup) {
	jobs.Go(func() {
		reporter := newLoopReporter(db, node, "agent", "agent", 30*time.Second, true)
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for ctx.Err() == nil {
			status, err := fetchAgentStatus(ctx, base)
			if err != nil {
				reportLoop(ctx, reporter, "agent status", err)
			} else {
				reporter.beat.Version = status.Version
				executions, marshalErr := json.Marshal(status.Executions)
				if marshalErr != nil {
					reportLoop(ctx, reporter, "agent status", marshalErr)
				} else {
					reporter.record(ctx, nil, executions)
				}
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	})
}

func startDailySnapshotLoop(ctx context.Context, db *pgxpool.Pool, node, role string, jobs *sync.WaitGroup) {
	jobs.Go(func() {
		reporter := newLoopReporter(db, node, role, "daily-snapshot", time.Minute, false)
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for ctx.Err() == nil {
			day, due := ops.BeijingSnapshotDay(time.Now())
			var err error
			if due {
				err = ops.TakeDailySnapshot(ctx, db, day)
			}
			reportLoop(ctx, reporter, "daily snapshot", err)
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	})
}
