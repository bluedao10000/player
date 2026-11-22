package knowledge

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	opsread "anime-agent/api/internal/repository/queries/ops"
	"anime-agent/api/internal/repository/workbench"
)

// The usage endpoints use the same half-open UTC window and server-side admin guard.
func usageRoutes(g *gin.RouterGroup, db *pgxpool.Pool) {
	g.GET("/ops/overview", func(c *gin.Context) {
		ctx, cancel := readContext(c)
		defer cancel()
		body, err := opsread.Overview(ctx, db)
		writeJSON(c, body, err)
	})
	g.GET("/ops/attention", func(c *gin.Context) {
		ctx, cancel := readContext(c)
		defer cancel()
		body, err := opsread.Attention(ctx, db)
		writeJSON(c, body, err)
	})
	g.GET("/backup-status", func(c *gin.Context) {
		ctx, cancel := readContext(c)
		defer cancel()
		body, err := workbench.BackupStatus(ctx, db)
		writeJSON(c, body, err)
	})
	g.GET("/episode-queue", func(c *gin.Context) {
		ctx, cancel := readContext(c)
		defer cancel()
		body, err := workbench.EpisodeQueue(ctx, db)
		writeJSON(c, body, err)
	})
	g.GET("/reader-releases", func(c *gin.Context) {
		ctx, cancel := readContext(c)
		defer cancel()
		body, err := workbench.ReaderReleases(ctx, db)
		writeJSON(c, body, err)
	})
	g.GET("/episode-inventory", func(c *gin.Context) {
		ctx, cancel := readContext(c)
		defer cancel()
		body, err := workbench.EpisodeInventory(ctx, db)
		writeJSON(c, body, err)
	})
	g.GET("/production-usage", func(c *gin.Context) {
		n := limit(c)
		if n > 0 {
			ctx, cancel := readContext(c)
			defer cancel()
			body, err := workbench.ProductionUsage(ctx, db, n)
			writeJSON(c, body, err)
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
		ctx, cancel := readContext(c)
		defer cancel()
		body, err := workbench.ModelUsage(ctx, db, from, to)
		writeJSON(c, body, err)
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
		ctx, cancel := readContext(c)
		defer cancel()
		body, err := workbench.Calls(ctx, db, from, to, bound, c.Query("subject"), n)
		writeJSON(c, body, err)
	})
}
