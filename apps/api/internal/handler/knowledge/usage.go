package knowledge

import (
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	knowledgeread "anime-agent/api/internal/repository/queries/knowledge"
	opsread "anime-agent/api/internal/repository/queries/ops"
	"anime-agent/api/internal/repository/workbench"
)

// The usage endpoints use the same half-open UTC window and server-side admin guard.
func usageRoutes(g *gin.RouterGroup, db *pgxpool.Pool) {
	g.GET("/ops/knowledge/summary", func(c *gin.Context) {
		rangeDays := 30
		switch c.DefaultQuery("range", "30d") {
		case "7d":
			rangeDays = 7
		case "30d":
		case "90d":
			rangeDays = 90
		default:
			c.JSON(400, gin.H{"error": "range must be 7d, 30d, or 90d"})
			return
		}
		ctx, cancel := readContext(c)
		defer cancel()
		body, err := knowledgeread.OpsKnowledgeSummary(ctx, db, rangeDays)
		writeJSON(c, body, err)
	})
	g.GET("/ops/knowledge/completeness", func(c *gin.Context) {
		work := strings.TrimSpace(c.Query("world"))
		if work == "" || len(work) > 512 {
			c.JSON(400, gin.H{"error": "world is required"})
			return
		}
		ctx, cancel := readContext(c)
		defer cancel()
		body, err := knowledgeread.OpsKnowledgeCompleteness(ctx, db, work)
		writeJSON(c, body, err)
	})
	g.GET("/ops/knowledge/works", func(c *gin.Context) {
		n := limit(c)
		if n == 0 {
			return
		}
		q := strings.TrimSpace(c.Query("q"))
		hasVersion := c.DefaultQuery("has_knowledge_version", "true")
		workType := c.Query("type")
		completenessStatus := c.Query("completeness_status")
		productionStatus := c.Query("production_status")
		sort := c.DefaultQuery("sort", "last_updated_at")
		allowedSort := map[string]bool{"title": true, "type": true, "release_versions": true, "episodes_covered": true,
			"characters": true, "scenes": true, "images": true, "completeness": true, "acceptance_pass_rate": true, "last_updated_at": true}
		if len(q) > 512 || len(workType) > 64 || (hasVersion != "true" && hasVersion != "false" && hasVersion != "all") ||
			!allowedSort[sort] || (completenessStatus != "" && completenessStatus != "completed" && completenessStatus != "partial" &&
			completenessStatus != "missing" && completenessStatus != "not_applicable" && completenessStatus != "pending_verification") ||
			(productionStatus != "" && productionStatus != "idle" && productionStatus != "pending" && productionStatus != "blocked" && productionStatus != "waiting_human") {
			c.JSON(400, gin.H{"error": "invalid work filter or sort"})
			return
		}
		after := 0
		if raw := c.Query("after"); raw != "" {
			value, err := strconv.Atoi(raw)
			if err != nil || value < 0 {
				c.JSON(400, gin.H{"error": "after must be a non-negative cursor"})
				return
			}
			after = value
		}
		ctx, cancel := readContext(c)
		defer cancel()
		body, err := knowledgeread.OpsKnowledgeWorks(ctx, db, q, hasVersion, workType, completenessStatus, productionStatus, sort, after, n)
		writeJSON(c, body, err)
	})
	g.GET("/ops/knowledge/works/:work", func(c *gin.Context) {
		work := strings.TrimSpace(c.Param("work"))
		if work == "" || len(work) > 512 {
			c.Status(400)
			return
		}
		ctx, cancel := readContext(c)
		defer cancel()
		body, err := knowledgeread.OpsKnowledgeWork(ctx, db, work)
		writeJSON(c, body, err)
	})
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
