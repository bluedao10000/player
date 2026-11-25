package workbench

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	q "anime-agent/api/internal/repository/queries/workbench"
	"anime-agent/api/internal/service/modelusage"
)

func (plan proposedPlan) validate(workID string) (string, string, []gin.H, error) {
	title, objective := strings.TrimSpace(plan.Title), strings.TrimSpace(plan.Objective)
	if workID == "" || !strings.EqualFold(workID, plan.WorkID) || title == "" || len(title) > 360 ||
		objective == "" || len(objective) > 1600 || len(plan.Tasks) == 0 || len(plan.Tasks) > 12 {
		return "", "", nil, errors.New("invalid agent plan")
	}
	tasks := make([]gin.H, 0, len(plan.Tasks))
	for i, task := range plan.Tasks {
		label := strings.TrimSpace(task.Title)
		if label == "" || len(label) > 480 || !oneOf(task.Priority, "high", "mid", "low") {
			return "", "", nil, errors.New("invalid agent plan task")
		}
		tasks = append(tasks, gin.H{"task_id": fmt.Sprintf("t-%d", i+1), "title": label,
			"priority": task.Priority, "estimated_cost": nil, "status": "pending", "dependencies": []string{}})
	}
	return title, objective, tasks, nil
}

func savePlan(ctx context.Context, tx pgx.Tx, runID string, plan *proposedPlan, workID string) ([]gin.H, error) {
	title, objective, tasks, err := plan.validate(workID)
	if err != nil {
		return nil, err
	}
	var planID, decisionID string
	if err = tx.QueryRow(ctx, q.Write["CreatePlan"], plan.WorkID, runID, title, objective, []any{}, tasks).Scan(&planID); err != nil {
		return nil, err
	}
	if err = tx.QueryRow(ctx, q.Write["CreatePlanDecision"], plan.WorkID, planID, title, objective).Scan(&decisionID); err != nil {
		return nil, err
	}
	return []gin.H{
		{"type": "artifact", "artifact": gin.H{"type": "plan", "ref": planID}},
		{"type": "artifact", "artifact": gin.H{"type": "approval", "ref": decisionID}},
		{"type": "follow_ups", "items": []string{"说明这份计划优先处理哪些缺口"}},
	}, nil
}

func (s *agentStream) finish(state, message string, input agentInput, result agentResult) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(s.c.Request.Context()), 8*time.Second)
	defer cancel()
	if receipt := result.receipt; receipt != nil && receipt.Purpose == "content" &&
		receipt.Operation == "conversation" && receipt.Subject == "workbench:"+s.id && receipt.Finished != nil {
		if err := modelusage.Save(ctx, s.handler.db, *receipt); err != nil {
			log.Printf("workbench model receipt %s: %v", s.id, err)
		}
	}
	finish := func(status, reason string) ([]gin.H, error) {
		events := []gin.H{}
		err := pgx.BeginFunc(ctx, s.handler.db, func(tx pgx.Tx) error {
			var cancellationRequested bool
			if err := tx.QueryRow(ctx, q.Write["LockRunCancellation"], s.id).Scan(&cancellationRequested); err != nil {
				return err
			}
			if cancellationRequested {
				status, reason = "cancelled", "本次生成已停止。"
			}
			var cost *string
			if err := tx.QueryRow(ctx, q.Read["ChatCost"], s.id).Scan(&cost); err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
			var references []gin.H
			if status == "completed" {
				var err error
				references, err = saveArtifactEvents(ctx, tx, s.id, input, result.artifacts)
				if err != nil {
					return err
				}
				if result.plan == nil && len(references) == 0 && strings.TrimSpace(result.text.String()) == "" {
					status, reason = "failed", "Agent 没有返回可用的结果。"
				}
			}
			if status == "completed" && result.plan != nil {
				var err error
				events, err = savePlan(ctx, tx, s.id, result.plan, input.PlanWorkID)
				if err != nil {
					return err
				}
				status = "waiting_approval"
			}
			if status == "completed" || status == "waiting_approval" {
				events = append(append([]gin.H{}, references...), events...)
			}
			terminal := gin.H{"type": "done", "run_id": s.id}
			switch status {
			case "cancelled":
				terminal["status"], terminal["summary"] = "cancelled", reason
			case "failed":
				terminal = gin.H{"type": "error", "code": "upstream_unavailable", "message": reason, "retryable": true}
			default:
				if result.input != nil || result.output != nil || cost != nil {
					usage := gin.H{"input_tokens": result.input, "output_tokens": result.output, "cost": nil, "cost_usd": nil, "currency": nil}
					if cost != nil {
						usage["cost"], usage["cost_usd"], usage["currency"] = json.Number(*cost), json.Number(*cost), "USD"
					}
					terminal["usage"] = usage
				}
			}
			events = append(events, terminal)
			for i, event := range events {
				if _, err := tx.Exec(ctx, q.Write["RecordEvent"], s.id, s.sequence+i+1, event); err != nil {
					return err
				}
			}
			summary := shorten(result.text.String(), 1200)
			if summary == "" {
				if status == "waiting_approval" {
					summary = "已生成知识计划，等待审批。"
				} else if status == "completed" && len(references) > 0 {
					summary = "已返回工作台数据引用。"
				}
			}
			tag, err := tx.Exec(ctx, q.Write["FinishRun"], s.id, status, result.input, result.output,
				result.toolCount, summary, reason, cost)
			if err == nil && tag.RowsAffected() != 1 {
				err = errors.New("workbench run already finalized")
			}
			return err
		})
		return events, err
	}
	events, err := finish(state, message)
	if err != nil && state == "completed" {
		log.Printf("workbench plan or run %s: %v", s.id, err)
		state = "failed"
		events, err = finish(state, "Agent 结果暂时无法保存。")
	}
	if err != nil {
		log.Printf("workbench run %s finalization: %v", s.id, err)
		if state != "cancelled" {
			_ = s.send(gin.H{"type": "error", "code": "upstream_unavailable", "message": "执行记录暂时无法保存。", "retryable": true})
		}
		return
	}
	if s.c.Request.Context().Err() == nil {
		for _, event := range events {
			if s.send(event) != nil {
				break
			}
		}
	}
}
