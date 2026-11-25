// Package workbench serves the operator API defined in docs/api-contract.md.
package workbench

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	catalog "anime-agent/api/internal/model/knowledge"
	"anime-agent/api/internal/repository/database"
	q "anime-agent/api/internal/repository/queries/workbench"
)

var beijing = time.FixedZone("Asia/Shanghai", 8*60*60)

type handler struct {
	db *pgxpool.Pool
}

type apiError struct {
	status  int
	code    string
	message string
	details any
}

func (e *apiError) Error() string { return e.message }

func invalid(field string) *apiError {
	return &apiError{400, "invalid_argument", "请求参数无效。", gin.H{"field": field}}
}

func conflict(revision int) *apiError {
	return &apiError{409, "revision_conflict", fmt.Sprintf("提案已变更（rev %d），请重新比较。", revision), gin.H{"current_revision": revision}}
}

func apiFailure(c *gin.Context, err error) {
	var problem *apiError
	if !errors.As(err, &problem) {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			problem = &apiError{404, "not_found", "没有找到请求的记录。", nil}
		default:
			log.Printf("workbench %s: %v", c.FullPath(), err)
			problem = &apiError{503, "upstream_unavailable", "数据服务暂不可用，请稍后重试。", nil}
		}
	}
	body := gin.H{"code": problem.code, "message": problem.message}
	if problem.details != nil {
		body["details"] = problem.details
	}
	c.AbortWithStatusJSON(problem.status, gin.H{"error": body})
}

func Routes(r *gin.Engine, db *pgxpool.Pool, authenticate func(string) string) {
	g := r.Group("/api/v1", func(c *gin.Context) {
		c.Header("Cache-Control", "private, no-store")
		operator := authenticate(c.GetHeader("Authorization"))
		if operator == "" {
			c.Header("WWW-Authenticate", "Bearer")
			apiFailure(c, &apiError{401, "unauthenticated", "请使用运营账号凭据。", nil})
			return
		}
		c.Set("operator", operator)
	}, gin.CustomRecovery(func(c *gin.Context, _ any) {
		if !c.Writer.Written() {
			apiFailure(c, &apiError{500, "internal", "服务暂时无法处理此请求。", nil})
		}
		c.Abort()
	}))
	h := handler{db: db}
	g.GET("/briefing", h.briefing)
	g.GET("/decisions", h.decisions)
	g.GET("/decisions/:id", h.uuidRead("Decision"))
	g.POST("/decisions/:id/resolve", h.resolve)
	g.GET("/works", h.works)
	g.GET("/works/:id", h.uuidRead("Work"))
	g.GET("/works/:id/profile", h.uuidRead("WorkProfile"))
	g.GET("/stats/overview", func(c *gin.Context) { h.read(c, "StatsOverview") })
	g.GET("/stats/knowledge-delta", h.knowledgeDelta)
	g.GET("/trends", h.trends)
	g.GET("/works/:id/gaps", h.workCollection("WorkGaps"))
	g.GET("/works/:id/recommendations", h.workCollection("WorkRecommendations"))
	g.GET("/runs", h.runs)
	g.GET("/runs/stats", h.runsStats)
	g.GET("/artifacts/:type/:ref", h.artifactResult)
	g.GET("/runs/:id", h.run)
	g.GET("/runs/:id/events", h.runEvents)
	g.GET("/evidence", func(c *gin.Context) { h.read(c, "Evidence") })
	g.GET("/plans/:id", h.uuidRead("Plan"))
	g.POST("/agent/runs", h.agentRun)
	g.GET("/agent/runs/:id", h.agentState)
	g.POST("/agent/runs/:id/cancel", h.cancelAgentRun)
}

func respond(c *gin.Context, value []byte, err error) {
	if err != nil {
		apiFailure(c, err)
		return
	}
	c.Data(http.StatusOK, "application/json; charset=utf-8", value)
}

func (h handler) read(c *gin.Context, name string, args ...any) {
	value, err := h.readJSON(c, name, args...)
	respond(c, value, err)
}

func (h handler) readJSON(c *gin.Context, name string, args ...any) ([]byte, error) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	return database.ReadJSON(ctx, h.db, q.Read[name], args...)
}

func (h handler) uuidRead(name string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if id, ok := uuidParam(c); ok {
			h.read(c, name, id)
		}
	}
}

func uuidParam(c *gin.Context) (string, bool) {
	id := c.Param("id")
	if !catalog.ValidUUID(id) {
		apiFailure(c, pgx.ErrNoRows)
		return "", false
	}
	return id, true
}

func (h handler) workCollection(name string) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := uuidParam(c)
		if !ok {
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()
		var value []byte
		err := pgx.BeginTxFunc(ctx, h.db, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly}, func(tx pgx.Tx) error {
			if _, err := database.ReadJSON(ctx, tx, q.Read["WorkExists"], id); err != nil {
				return err
			}
			var err error
			value, err = database.ReadJSON(ctx, tx, q.Read[name], id)
			return err
		})
		// The read-only transaction keeps the existence and collection checks on one snapshot.
		respond(c, value, err)
	}
}

func oneOf(value string, choices ...string) bool {
	for _, choice := range choices {
		if choice == value {
			return true
		}
	}
	return false
}

func pageNumber(c *gin.Context, field string, fallback, min, max int) (int, error) {
	raw, present := c.GetQuery(field)
	if !present {
		return fallback, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < min || n > max {
		return 0, invalid(field)
	}
	return n, nil
}

func (h handler) works(c *gin.Context) {
	search, kind := strings.TrimSpace(c.Query("search")), strings.TrimSpace(c.Query("type"))
	status, sort := c.Query("status"), c.DefaultQuery("sort", "updated")
	switch {
	case len(search) > 512:
		apiFailure(c, invalid("search"))
	case len(kind) > 80:
		apiFailure(c, invalid("type"))
	case !oneOf(status, "", "published", "draft", "archived"):
		apiFailure(c, invalid("status"))
	case !oneOf(sort, "updated", "gap", "coverage"):
		apiFailure(c, invalid("sort"))
	default:
		page, err := pageNumber(c, "page", 0, 0, 1_000_000)
		if err != nil {
			apiFailure(c, err)
			return
		}
		size, err := pageNumber(c, "page_size", 10, 1, 100)
		if err != nil {
			apiFailure(c, err)
			return
		}
		h.workPage(c, search, kind, status, sort, size, page*size)
	}
}

func (h handler) workPage(c *gin.Context, search, kind, status, sort string, size, offset int) {
	ref := c.Query("ref")
	if ref != "" && !catalog.ValidUUID(ref) {
		apiFailure(c, invalid("ref"))
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	var result []byte
	err := pgx.BeginTxFunc(ctx, h.db, pgx.TxOptions{AccessMode: pgx.ReadOnly}, func(tx pgx.Tx) error {
		var ids []string
		if ref != "" {
			body, err := database.ReadJSON(ctx, tx, q.Read["WorkSet"], ref, c.GetString("operator"))
			if err != nil {
				return err
			}
			if err = json.Unmarshal(body, &ids); err != nil {
				return err
			}
		}
		// Plan the catalog query after setting JIT locally. Every other query
		// retains the connection's default when this transaction ends.
		if _, err := tx.Exec(ctx, q.Read["DisableCatalogJIT"]); err != nil {
			return err
		}
		var err error
		result, err = database.ReadJSON(ctx, tx, q.Read["Works"], pgx.QueryExecModeExec,
			search, kind, status, sort, size, offset, ids)
		return err
	})
	respond(c, result, err)
}

func (h handler) decisions(c *gin.Context) {
	status, risk := c.Query("status"), c.Query("risk")
	if !oneOf(status, "", "pending", "approved", "rejected", "snoozed", "expired") {
		apiFailure(c, invalid("status"))
		return
	}
	if !oneOf(risk, "", "L1", "L2", "L3") {
		apiFailure(c, invalid("risk"))
		return
	}
	h.read(c, "Decisions", status, risk)
}

func (h handler) runs(c *gin.Context) {
	status := c.Query("status")
	if !oneOf(status, "", "queued", "running", "completed", "failed", "cancelled", "waiting_approval") {
		apiFailure(c, invalid("status"))
		return
	}
	ref := c.Query("ref")
	if len(ref) > 256 {
		apiFailure(c, invalid("ref"))
		return
	}
	h.read(c, "Runs", status, ref)
}

func (h handler) run(c *gin.Context) {
	h.read(c, "Run", c.Param("id"))
}

func (h handler) runEvents(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	id := c.Param("id")
	if _, err := database.ReadJSON(ctx, h.db, q.Read["RunExists"], id); err != nil {
		apiFailure(c, err)
		return
	}
	value, err := database.ReadJSON(ctx, h.db, q.Read["RunEvents"], id)
	respond(c, value, err)
}

func (h handler) agentState(c *gin.Context) {
	if id, ok := uuidParam(c); ok {
		h.read(c, "AgentRun", id, c.GetString("operator"))
	}
}

func (h handler) briefing(c *gin.Context) {
	now := time.Now().In(beijing)
	date := c.DefaultQuery("date", now.Format(time.DateOnly))
	start, err := time.ParseInLocation(time.DateOnly, date, beijing)
	if err != nil || start.After(now) {
		apiFailure(c, invalid("date"))
		return
	}
	cutoff := start.AddDate(0, 0, 1)
	today := cutoff.After(now)
	if today {
		cutoff = now
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	body, err := database.ReadJSON(ctx, h.db, q.Read["Briefing"], start, cutoff, today)
	if err != nil {
		apiFailure(c, err)
		return
	}
	var detail struct {
		Generated string          `json:"generated_at"`
		DataUntil string          `json:"data_until"`
		Metrics   json.RawMessage `json:"metrics"`
		Risks     json.RawMessage `json:"risks"`
		Actions   json.RawMessage `json:"actions"`
		Execution json.RawMessage `json:"execution"`
		Observed  int
		Completed int
		Failed    int
		Cancelled int
	}
	if err = json.Unmarshal(body, &detail); err != nil {
		apiFailure(c, err)
		return
	}
	summary := []string{"截至简报时点，尚无当日执行记录。"}
	if detail.Observed > 0 {
		summary = []string{fmt.Sprintf("当日记录 %d 次执行，", detail.Observed),
			fmt.Sprintf("其中 %d 次已完成，%d 次失败，", detail.Completed, detail.Failed),
			fmt.Sprintf("%d 次手动停止。", detail.Cancelled)}
	}
	c.JSON(200, gin.H{"generated_at": detail.Generated, "data_until": detail.DataUntil, "summary_tokens": summary,
		"metrics": detail.Metrics, "risks": detail.Risks, "actions": detail.Actions, "execution": detail.Execution})
}
