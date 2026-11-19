package billing

import (
	"crypto/rand"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed reconciliation.sql
var reconciliation string
var currency = regexp.MustCompile(`^[A-Z]{3}$`)

func fail(c *gin.Context, err error) {
	var p *pgconn.PgError
	switch {
	case errors.Is(err, ErrInvalidStatement):
		c.JSON(400, gin.H{"error": err.Error()})
	case errors.Is(err, pgx.ErrNoRows):
		c.JSON(404, gin.H{"error": "not found"})
	case errors.Is(err, ErrConflict):
		c.JSON(409, gin.H{"error": err.Error()})
	case errors.Is(err, ErrPending):
		c.JSON(409, gin.H{"error": err.Error(), "state": "pending_reconciliation"})
	case errors.Is(err, ErrWorkerOffline):
		c.JSON(503, gin.H{"error": err.Error()})
	case errors.Is(err, ErrFunds):
		c.JSON(402, gin.H{"error": err.Error()})
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
func orderInfo(c *gin.Context, db *pgxpool.Pool, id, owner string) {
	jsonRow(c, db, `SELECT to_jsonb(o) FROM billing_order_details o WHERE id=$1 AND ($2='' OR account_id=nullif($2,'')::uuid)`, id, owner)
}
func listOrders(c *gin.Context, db *pgxpool.Pool, owner string) {
	before, err := strconv.ParseInt(c.DefaultQuery("before", "9223372036854775807"), 10, 64)
	if err != nil || before < 1 {
		c.Status(400)
		return
	}
	jsonRow(c, db, `SELECT coalesce(jsonb_agg(to_jsonb(x) ORDER BY seq DESC),'[]') FROM (
 SELECT seq,id,account_id,status,created_at,finished_at,currency,cap_micros,charge_micros,request#>>'{brief,original_subject}' AS subject FROM billing_order_details
 WHERE ($1='' OR account_id=nullif($1,'')::uuid) AND ($2='' OR status=$2) AND seq<$3 ORDER BY seq DESC LIMIT 100)x`, owner, c.Query("status"), before)
}

func customerAuth(db *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		raw, ok := strings.CutPrefix(c.GetHeader("Authorization"), "Bearer ")
		if !ok || len(raw) < 32 || len(raw) > 512 {
			c.AbortWithStatus(401)
			return
		}
		h := sha256.Sum256([]byte(raw))
		var id string
		var isSession bool
		err := db.QueryRow(c.Request.Context(), `SELECT id::text,false FROM billing_accounts WHERE token_hash=$1
 UNION ALL SELECT account_id::text,true FROM billing_reader_sessions WHERE token_hash=$1 AND expires_at>now()`, h[:]).Scan(&id, &isSession)
		if errors.Is(err, pgx.ErrNoRows) {
			c.AbortWithStatus(401)
			return
		}
		if err != nil {
			fail(c, err)
			c.Abort()
			return
		}
		c.Set("billing_account", id)
		c.Set("billing_session", isSession)
		c.Set("billing_hash", h[:])
		c.Next()
	}
}

func CustomerRoutes(r *gin.Engine, db *pgxpool.Pool) {
	g := r.Group("/api/v1/billing", customerAuth(db))
	alipayEnabled := alipayRoutes(r, g, db)
	owner := func(c *gin.Context) string { return c.GetString("billing_account") }
	g.POST("/session", func(c *gin.Context) {
		if c.GetBool("billing_session") {
			c.Status(http.StatusForbidden)
			return
		}
		var secret [32]byte
		rand.Read(secret[:])
		token := hex.EncodeToString(secret[:])
		hash := sha256.Sum256([]byte(token))
		_, err := db.Exec(c.Request.Context(), `WITH cleanup AS (DELETE FROM billing_reader_sessions WHERE expires_at<=now())
 INSERT INTO billing_reader_sessions(token_hash,account_id) VALUES($1,$2)`, hash[:], owner(c))
		if err != nil {
			fail(c, err)
			return
		}
		c.JSON(200, gin.H{"token": token})
	})
	g.DELETE("/session", func(c *gin.Context) {
		hash, _ := c.Get("billing_hash")
		if _, err := db.Exec(c.Request.Context(), "DELETE FROM billing_reader_sessions WHERE token_hash=$1", hash); err != nil {
			fail(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	})
	g.GET("/account", func(c *gin.Context) {
		jsonRow(c, db, "SELECT to_jsonb(b)||jsonb_build_object('alipay_available',$2::boolean AND currency='CNY') FROM billing_balances b WHERE id=$1", owner(c), alipayEnabled)
	})
	g.GET("/quote", func(c *gin.Context) {
		jsonRow(c, db, `SELECT jsonb_build_object('product','image-preview','automatic',EXISTS(SELECT FROM billing_preview_workers w WHERE (w.endpoint,w.provider_account,w.model)=(r.endpoint,r.provider_account,r.model) AND w.available_until>now()),'rate',to_jsonb(r)-ARRAY['created_by','provider_account','endpoint'])
 FROM billing_rates r JOIN billing_accounts a ON a.currency=r.currency WHERE a.id=$1 AND r.kind='retail' AND effective_at<=now() ORDER BY effective_at DESC LIMIT 1`, owner(c))
	})
	g.GET("/entries", func(c *gin.Context) {
		before, e := strconv.ParseInt(c.DefaultQuery("before", "9223372036854775807"), 10, 64)
		if e != nil || before <= 0 {
			c.Status(400)
			return
		}
		jsonRow(c, db, `SELECT coalesce(jsonb_agg(to_jsonb(x) ORDER BY id DESC),'[]') FROM
  (SELECT id,kind,order_id,amount_micros,at FROM billing_entries WHERE account_id=$1 AND id<$2 ORDER BY id DESC LIMIT 100)x`, owner(c), before)
	})
	g.POST("/orders", func(c *gin.Context) {
		var v struct {
			Request PreviewRequest `json:"request" binding:"required"`
			Key     string         `json:"client_key" binding:"required,max=128"`
			Rate    string         `json:"rate_id" binding:"required,uuid"`
		}
		if !input(c, &v) {
			return
		}
		id, e := Reserve(c.Request.Context(), db, owner(c), v.Key, v.Rate, v.Request, c.GetBool("billing_session"))
		if e != nil {
			fail(c, e)
			return
		}
		orderInfo(c, db, id, owner(c))
	})
	g.GET("/orders", func(c *gin.Context) { listOrders(c, db, owner(c)) })
	g.GET("/orders/:id", func(c *gin.Context) { orderInfo(c, db, c.Param("id"), owner(c)) })
	g.GET("/orders/:id/image", func(c *gin.Context) {
		var id string
		if e := db.QueryRow(c.Request.Context(), "SELECT id::text FROM billing_orders WHERE id=$1 AND account_id=$2 AND status='settled'", c.Param("id"), owner(c)).Scan(&id); e != nil {
			fail(c, e)
			return
		}
		c.Header("X-Content-Type-Options", "nosniff")
		c.FileAttachment(PreviewPath(id), "preview.png")
	})
	g.POST("/orders/:id/cancel", func(c *gin.Context) {
		if e := Change(c.Request.Context(), db, c.Param("id"), owner(c), "cancel", "customer", "cancelled by account"); e != nil {
			fail(c, e)
			return
		}
		orderInfo(c, db, c.Param("id"), owner(c))
	})
}

type rateInput struct {
	Kind       string    `json:"kind" binding:"required,oneof=retail cost"`
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
	g.POST("/accounts", func(c *gin.Context) {
		var v struct {
			Ref      string `json:"customer_ref" binding:"required,max=150"`
			Currency string `json:"currency" binding:"required,len=3"`
			Hash     string `json:"token_sha256" binding:"required,len=64"`
		}
		if !input(c, &v) {
			return
		}
		h, e := hex.DecodeString(v.Hash)
		if e != nil {
			c.Status(400)
			return
		}
		jsonRow(c, db, `INSERT INTO billing_accounts(customer_ref,currency,token_hash,created_by) VALUES($1,$2,$3,$4)
   RETURNING jsonb_build_object('id',id,'customer_ref',customer_ref,'currency',currency)`, v.Ref, v.Currency, h, c.GetString("operator"))
	})
	g.POST("/accounts/:id/credits", func(c *gin.Context) {
		var v struct {
			Ref      string `json:"reference" binding:"required,max=128"`
			Amount   int64  `json:"amount_micros" binding:"gt=0"`
			Currency string `json:"currency" binding:"required,len=3"`
		}
		if !input(c, &v) {
			return
		}
		if e := Credit(c.Request.Context(), db, c.Param("id"), v.Ref, c.GetString("operator"), v.Currency, v.Amount); e != nil {
			fail(c, e)
			return
		}
		jsonRow(c, db, "SELECT to_jsonb(b) FROM billing_balances b WHERE id=$1", c.Param("id"))
	})
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
		jsonRow(c, db, `INSERT INTO billing_rates(kind,endpoint,provider_account,model,currency,effective_at,unit,unit_micros,input_micros,output_micros,cached_micros,cap_micros,created_by)
  VALUES($1,$2,$3,$4,$5,$6,$7,coalesce($8,0::bigint),coalesce($9,0::bigint),coalesce($10,0::bigint),$11,$12,$13) RETURNING jsonb_build_object('id',id,'kind',kind,'effective_at',effective_at)`,
			v.Kind, strings.TrimRight(v.Endpoint, "/"), v.Account, v.Model, v.Currency, v.Effective, v.Unit, v.UnitRate, v.InputRate, v.OutputRate, v.Cached, v.Cap, c.GetString("operator"))
	})
	g.GET("/orders", func(c *gin.Context) { listOrders(c, db, "") })
	g.GET("/orders/:id", func(c *gin.Context) { orderInfo(c, db, c.Param("id"), "") })
	g.POST("/orders/:id/:action", func(c *gin.Context) {
		action := c.Param("action")
		var v struct {
			Reason string `json:"reason" binding:"required,max=500"`
		}
		if action != "settle" {
			if action != "refund" && action != "cancel" {
				c.Status(404)
				return
			}
			if !input(c, &v) {
				return
			}
		}
		var e error
		if action == "settle" {
			e = FinalizePreview(c.Request.Context(), db, c.Param("id"), c.GetString("operator"))
		} else {
			e = Change(c.Request.Context(), db, c.Param("id"), "", action, c.GetString("operator"), v.Reason)
		}
		if e != nil {
			fail(c, e)
			return
		}
		orderInfo(c, db, c.Param("id"), "")
	})
	g.POST("/statements", func(c *gin.Context) {
		var v statement
		if !input(c, &v) {
			return
		}
		v.Endpoint = strings.TrimRight(v.Endpoint, "/")
		if e := importStatement(c.Request.Context(), db, v, c.GetString("operator")); e != nil {
			fail(c, e)
			return
		}
		c.Status(204)
	})
	g.GET("/rates", func(c *gin.Context) {
		jsonRow(c, db, `SELECT coalesce(jsonb_agg(to_jsonb(x) ORDER BY effective_at DESC,id),'[]') FROM
 (SELECT * FROM billing_rates WHERE ($1='' OR kind=$1) AND ($2='' OR currency=$2) ORDER BY effective_at DESC,id LIMIT 100)x`, c.Query("kind"), c.Query("currency"))
	})
	g.GET("/statements", func(c *gin.Context) {
		day, cur, ok := reportWindow(c)
		if !ok {
			return
		}
		jsonRow(c, db, `SELECT coalesce(jsonb_agg(to_jsonb(s) ORDER BY id),'[]') FROM billing_statements s WHERE day=$1 AND currency=$2`, day, cur)
	})
	g.GET("/reconciliation/receipts", func(c *gin.Context) {
		day, cur, ok := reportWindow(c)
		if !ok {
			return
		}
		jsonRow(c, db, `SELECT coalesce(jsonb_agg(to_jsonb(x) ORDER BY call_id),'[]') FROM
 (SELECT * FROM billing_daily_costs($1,$2) WHERE ($3='' OR call_id>nullif($3,'')::uuid)
 AND ($4='' OR provider_account=$4) AND ($5='' OR model=$5) ORDER BY call_id LIMIT 100)x`, day, cur, c.Query("after"), c.Query("provider_account"), c.Query("model"))
	})
	g.GET("/reconciliation", func(c *gin.Context) {
		day, cur, ok := reportWindow(c)
		if !ok {
			return
		}
		jsonRow(c, db, reconciliation, day, cur)
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
