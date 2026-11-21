package billing

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	domain "anime-agent/api/internal/service/billing"
	queries "anime-agent/api/internal/repository/queries/billing"
)

var currency = regexp.MustCompile(`^[A-Z]{3}$`)

func fail(c *gin.Context, err error) {
	var p *pgconn.PgError
	switch {
	case errors.Is(err, domain.ErrInvalidStatement):
		c.JSON(400, gin.H{"error": err.Error()})
	case errors.Is(err, pgx.ErrNoRows):
		c.JSON(404, gin.H{"error": "not found"})
	case errors.Is(err, domain.ErrConflict):
		c.JSON(409, gin.H{"error": err.Error()})
	case errors.As(err, &p) && (p.Code == "23505" || p.Code == "23503"):
		c.JSON(409, gin.H{"error": "conflicting record"})
	case errors.As(err, &p) && (p.Code == "23514" || p.Code == "23502" || p.Code == "22P02" || p.Code == "22003"):
		c.JSON(400, gin.H{"error": "invalid billing parameters"})
	default:
		log.Printf("billing query failed: %v", err)
		c.JSON(503, gin.H{"error": "billing unavailable"})
	}
}
func input(c *gin.Context, v any) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16384)
	if e := c.ShouldBindJSON(v); e != nil {
		c.JSON(400, gin.H{"error": "invalid JSON or missing fields"})
		return false
	}
	return true
}
func jsonRow(c *gin.Context, db *pgxpool.Pool, sql string, args ...any) {
	var b json.RawMessage
	if err := db.QueryRow(c.Request.Context(), sql, args...).Scan(&b); err != nil {
		fail(c, err)
		return
	}
	c.Data(200, "application/json; charset=utf-8", b)
}

type rateInput struct {
	Kind       string    `json:"kind" binding:"required,eq=cost"`
	Endpoint   string    `json:"endpoint" binding:"required,max=512"`
	Account    string    `json:"provider_account" binding:"required,max=128"`
	Model      string    `json:"model" binding:"required,max=200"`
	Currency   string    `json:"currency" binding:"required,len=3"`
	Effective  time.Time `json:"effective_at" binding:"required"`
	Unit       string    `json:"unit" binding:"required,oneof=image total_tokens split_tokens"`
	UnitRate   *int64    `json:"unit_micros" binding:"omitempty,gte=0"`
	InputRate  *int64    `json:"input_micros" binding:"omitempty,gte=0"`
	OutputRate *int64    `json:"output_micros" binding:"omitempty,gte=0"`
	Cached     *int64    `json:"cached_micros"`
	Cap        int64     `json:"cap_micros" binding:"omitempty,gte=0"`
}

// AdminRoutes inherits the knowledge admin guard including operator audit identity.
func AdminRoutes(admin *gin.RouterGroup, db *pgxpool.Pool) {
	g := admin.Group("/billing")
	g.POST("/rates", func(c *gin.Context) {
		var v rateInput
		if !input(c, &v) {
			return
		}
		if (v.Unit == "split_tokens" && (v.InputRate == nil || v.OutputRate == nil || v.UnitRate != nil)) ||
			(v.Unit != "split_tokens" && (v.UnitRate == nil || v.InputRate != nil || v.OutputRate != nil || v.Cached != nil)) {
			c.JSON(400, gin.H{"error": "provide the explicit rates for the selected unit"})
			return
		}
		jsonRow(c, db, queries.CreateRate,
			v.Kind, strings.TrimRight(v.Endpoint, "/"), v.Account, v.Model, v.Currency, v.Effective, v.Unit, v.UnitRate, v.InputRate, v.OutputRate, v.Cached, v.Cap, c.GetString("operator"))
	})
	g.POST("/statements", func(c *gin.Context) {
		var v domain.Statement
		if !input(c, &v) {
			return
		}
		v.Endpoint = strings.TrimRight(v.Endpoint, "/")
		if e := domain.ImportStatement(c.Request.Context(), db, v, c.GetString("operator")); e != nil {
			fail(c, e)
			return
		}
		c.Status(204)
	})
	g.GET("/rates", func(c *gin.Context) {
		jsonRow(c, db, queries.ListRates, c.Query("kind"), c.Query("currency"))
	})
	g.GET("/statements", func(c *gin.Context) {
		day, cur, ok := reportWindow(c)
		if !ok {
			return
		}
		jsonRow(c, db, queries.DailyStatements, day, cur)
	})
	g.GET("/reconciliation/receipts", func(c *gin.Context) {
		day, cur, ok := reportWindow(c)
		if !ok {
			return
		}
		jsonRow(c, db, queries.DailyReceipts, day, cur, c.Query("after"), c.Query("provider_account"), c.Query("model"))
	})
	g.GET("/reconciliation", func(c *gin.Context) {
		day, cur, ok := reportWindow(c)
		if !ok {
			return
		}
		jsonRow(c, db, queries.Reconciliation, day, cur)
	})
}

func reportWindow(c *gin.Context) (string, string, bool) {
	day, cur := c.Query("day"), c.Query("currency")
	if _, err := time.Parse(time.DateOnly, day); err != nil || !currency.MatchString(cur) {
		c.Status(400)
		return "", "", false
	}
	return day, cur, true
}
