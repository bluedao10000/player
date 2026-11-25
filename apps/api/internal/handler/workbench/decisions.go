package workbench

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"anime-agent/api/internal/repository/database"
	q "anime-agent/api/internal/repository/queries/workbench"
)

type resolution struct {
	Action   string `json:"action"`
	Note     string `json:"note"`
	Revision int    `json:"expected_revision"`
}

func (h handler) resolve(c *gin.Context) {
	id, ok := uuidParam(c)
	if !ok {
		return
	}
	var input resolution
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10)
	if c.ShouldBindJSON(&input) != nil {
		apiFailure(c, invalid("body"))
		return
	}
	input.Note = strings.TrimSpace(input.Note)
	switch {
	case !oneOf(input.Action, "approve", "reject", "snooze"):
		apiFailure(c, invalid("action"))
		return
	case input.Revision < 1:
		apiFailure(c, invalid("expected_revision"))
		return
	case len(input.Note) > 8000 || input.Action == "reject" && input.Note == "":
		apiFailure(c, invalid("note"))
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	operator := c.GetString("operator")
	var body []byte
	err := pgx.BeginFunc(ctx, h.db, func(tx pgx.Tx) error {
		// Source ingestion holds the source mark first and revises its inbox item via the trigger.
		var locked int
		err := tx.QueryRow(ctx, q.Write["LockDecisionSource"], id).Scan(&locked)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		var revision int
		var state, actionType, work string
		var expired bool
		var plan, sourceRun *string
		var source *int64
		err = tx.QueryRow(ctx, q.Write["LockDecision"], id).Scan(&revision, &state, &expired, &actionType, &plan, &source, &sourceRun, &work)
		if err != nil {
			return err
		}
		if revision != input.Revision {
			return conflict(revision)
		}
		if state == "expired" || expired && oneOf(state, "pending", "snoozed") {
			return &apiError{410, "decision_expired", "此提案已过期。", nil}
		}
		if !oneOf(state, "pending", "snoozed") {
			return conflict(revision)
		}
		state = "snoozed"
		if input.Action != "snooze" {
			state = "rejected"
			if input.Action == "approve" {
				state = "approved"
			}
			var tag pgconn.CommandTag
			if actionType == "plan.approve" {
				next := "cancelled"
				if state == "approved" {
					next = "approved"
				}
				tag, err = tx.Exec(ctx, q.Write["UpdatePlanStatus"], plan, next)
			} else {
				tag, err = tx.Exec(ctx, q.Write["CloseSourceReview"], source, sourceRun, operator)
			}
			if err != nil {
				return err
			}
			if tag.RowsAffected() != 1 {
				return conflict(revision)
			}
			if plan != nil {
				if _, err = tx.Exec(ctx, q.Write["FinishPlanProposal"], plan); err != nil {
					return err
				}
			}
		}

		var runID *string
		if state == "approved" {
			title, summary := "来源变动核对", "来源变动已核对，处理结果已记录。"
			if actionType == "plan.approve" {
				title, summary = "知识计划审批", "知识计划已登记为已批准。"
			}
			if err = tx.QueryRow(ctx, q.Write["ApprovalRun"], work, title, actionType, summary).Scan(&runID); err != nil {
				return err
			}
		}
		if err = tx.QueryRow(ctx, q.Write["SaveDecision"], id, state, runID, operator, input.Note).Scan(&revision); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, q.Write["AuditDecision"], id, revision, operator, input.Action, input.Note); err != nil {
			return err
		}
		body, err = database.ReadJSON(ctx, tx, q.Read["Decision"], id)
		return err
	})
	respond(c, body, err)
}
