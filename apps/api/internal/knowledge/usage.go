package knowledge

import (
	_ "embed"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed production_usage.sql
var productionUsage string

// The usage endpoints use the same half-open UTC window and server-side admin guard.
func usageRoutes(g *gin.RouterGroup, db *pgxpool.Pool) {
	g.GET("/episode-queue", func(c *gin.Context) {
		queryJSON(c, db, `SELECT coalesce(jsonb_agg(to_jsonb(q)||jsonb_build_object('capture_failures',
  (SELECT jsonb_agg(jsonb_build_object('at',finished_at,'reason',error) ORDER BY finished_at)
   FROM episode_capture_attempts a WHERE a.topic=q.topic AND a.error IS NOT NULL)) ORDER BY priority DESC,number,world_slug),'[]') FROM episode_policy_queue q`)
	})
	g.GET("/episode-inventory", func(c *gin.Context) {
		queryJSON(c, db, `SELECT coalesce(jsonb_agg(jsonb_build_object('checked',checked_at,'works',works)),'[]') FROM (SELECT * FROM episode_inventory_snapshots ORDER BY checked_at DESC LIMIT 1)s`)
	})
	g.GET("/production-usage", func(c *gin.Context) {
		n := limit(c)
		if n > 0 {
			queryJSON(c, db, productionUsage, n)
		}
	})
	window := func(c *gin.Context) (from, to time.Time, ok bool) {
		to = time.Now().UTC()
		var err error
		if v := c.Query("to"); v != "" {
			to, err = time.Parse(time.RFC3339, v)
		}
		from = to.AddDate(0, 0, -30)
		if v := c.Query("from"); v != "" && err == nil {
			from, err = time.Parse(time.RFC3339, v)
		}
		if err != nil || !from.Before(to) {
			c.JSON(400, gin.H{"error": "expected from < to in RFC3339"})
			return
		}
		return from, to, true
	}
	g.GET("/model-usage", func(c *gin.Context) {
		from, to, ok := window(c)
		if !ok {
			return
		}
		queryJSON(c, db, `SELECT jsonb_build_object('from',$1::timestamptz,'to',$2::timestamptz,'groups',coalesce(jsonb_agg(to_jsonb(x) ORDER BY purpose,endpoint,provider_account,model),'[]'))
  FROM (SELECT purpose,endpoint,provider_account,coalesce(resolved_model,requested_model) model,count(*) attempts,
   count(*) FILTER(WHERE receipt_rank=1 AND meter_state='measured') measured,
   count(*) FILTER(WHERE receipt_rank>1) duplicates,
   count(*) FILTER(WHERE receipt_rank=1 AND meter_state<>'measured') unmetered,
   count(*) FILTER(WHERE outcome IN ('started','unknown')) unresolved,
   count(*) FILTER(WHERE receipt_rank=1 AND meter_state='inconsistent') inconsistent,
   sum(input_tokens) FILTER(WHERE receipt_rank=1 AND meter_state='measured') input_tokens,
   sum(output_tokens) FILTER(WHERE receipt_rank=1 AND meter_state='measured') output_tokens,
   sum(total_tokens) FILTER(WHERE receipt_rank=1 AND meter_state='measured') total_tokens,
   count(*) FILTER(WHERE receipt_rank=1 AND meter_state='partial') partial,
   count(*) FILTER(WHERE receipt_rank=1 AND meter_state='unreported') unreported,
   sum(images) FILTER(WHERE receipt_rank=1 AND meter_state<>'inconsistent') images
   FROM model_usage_receipts WHERE receipt_started_at >= $1 AND receipt_started_at < $2 GROUP BY 1,2,3,4)x`, from, to)
	})
	g.GET("/model-usage/calls", func(c *gin.Context) {
		from, to, ok := window(c)
		if !ok {
			return
		}
		bound, err := strconv.ParseInt(c.DefaultQuery("before", "9223372036854775807"), 10, 64)
		if err != nil || bound < 1 {
			c.Status(400)
			return
		}
		n := limit(c)
		if n == 0 {
			return
		}
		queryJSON(c, db, `SELECT coalesce(jsonb_agg(to_jsonb(x) ORDER BY seq DESC),'[]') FROM (
   SELECT * FROM model_usage_receipts WHERE started_at >= $1 AND started_at < $2 AND seq < $3 AND ($4='' OR subject=$4) ORDER BY seq DESC LIMIT $5)x`, from, to, bound, c.Query("subject"), n)
	})
}
