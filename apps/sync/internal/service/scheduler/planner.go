package scheduler

import (
	q "anime-agent/api/internal/repository/queries/knowledge"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"
)

func draftPlan(ctx context.Context, db *pgxpool.Pool, agent string) error {
	var id, call string
	var rev int
	var gaps []int32
	var payload []byte
	err := pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, q.ClaimPlanDraft).Scan(&id, &rev, &call, &gaps, &payload)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, q.MarkPlanDrafting, id)
		return err
	})
	if err != nil || id == "" {
		return err
	}
	endpoint := fmt.Sprintf("%s/internal/editor/%s/%d/1/planning", strings.TrimRight(agent, "/"), id, rev)
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	client := http.Client{Timeout: 3 * time.Minute}
	var reply struct {
		Tokens *int64
		Value  struct {
			Episodes []int32
			Brief    string
		}
	}
	stage, brief := "blocked", "规划调用缺少可核对的用量回执"
	resp, err := client.Do(req)
	if err == nil {
		defer resp.Body.Close()
		if resp.StatusCode == 200 && json.NewDecoder(io.LimitReader(resp.Body, 64000)).Decode(&reply) == nil && reply.Tokens != nil && *reply.Tokens > 0 {
			if slices.Equal(reply.Value.Episodes, gaps) && strings.TrimSpace(reply.Value.Brief) != "" {
				stage, brief = "plan_validated", reply.Value.Brief
			} else {
				brief = "规划结果与已确认的分集缺口不一致"
			}
		} else {
			reply.Tokens = nil
		}
	}
	return pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, q.SettlePlanDraft, id, call, reply.Tokens); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, q.SavePlanDraft, id, rev, stage, brief)
		return err
	})
}
