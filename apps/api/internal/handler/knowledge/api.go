package knowledge

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	catalog "anime-agent/api/internal/model/knowledge"
	reads "anime-agent/api/internal/repository/catalog"
)

func limit(c *gin.Context) int {
	n, e := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if e != nil || n < 1 || n > 100 {
		c.JSON(400, gin.H{"error": "limit must be 1..100"})
		return 0
	}
	return n
}
func readContext(c *gin.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(c.Request.Context(), 10*time.Second)
}
func writeJSON(c *gin.Context, result []byte, err error) {
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(404, gin.H{"error": "work not found"})
		return
	}
	if err != nil {
		log.Printf("knowledge query: %v", err)
		c.JSON(503, gin.H{"error": "knowledge database unavailable"})
		return
	}
	c.Data(http.StatusOK, "application/json; charset=utf-8", result)
}
func Routes(r *gin.Engine, db *pgxpool.Pool) {
	g := r.Group("/api/v1/knowledge")
	worldRoutes(g, db)
	mediaRoutes(g, db)
	g.GET("/works", func(c *gin.Context) {
		n := limit(c)
		if n == 0 {
			return
		}
		q := strings.TrimSpace(c.Query("q"))
		after := c.Query("after")
		if len(q) > 512 || (after != "" && !catalog.ValidUUID(after)) {
			c.JSON(400, gin.H{"error": "invalid q or after"})
			return
		}
		ctx, cancel := readContext(c)
		defer cancel()
		body, err := reads.Works(ctx, db, q, after, n)
		writeJSON(c, body, err)
	})
	g.GET("/works/:id", func(c *gin.Context) {
		id := c.Param("id")
		if !catalog.ValidUUID(id) {
			c.JSON(400, gin.H{"error": "invalid work UUID"})
			return
		}
		ctx, cancel := readContext(c)
		defer cancel()
		body, err := reads.Work(ctx, db, id)
		writeJSON(c, body, err)
	})
	// The workbench lists public world revisions. Catalog-only works can also
	// be inspected directly by their canonical UUID.
	g.GET("/research-runs", func(c *gin.Context) {
		n := limit(c)
		if n == 0 {
			return
		}
		ctx, cancel := readContext(c)
		defer cancel()
		body, err := reads.ResearchRuns(ctx, db, n)
		writeJSON(c, body, err)
	})
	g.GET("/world-statuses", func(c *gin.Context) {
		n := limit(c)
		if n == 0 {
			return
		}
		ctx, cancel := readContext(c)
		defer cancel()
		body, err := reads.WorldStatuses(ctx, db, n)
		writeJSON(c, body, err)
	})
	g.GET("/works/:id/status", func(c *gin.Context) {
		id := c.Param("id")
		if !catalog.ValidUUID(id) {
			c.JSON(400, gin.H{"error": "invalid work id"})
			return
		}
		ctx, cancel := readContext(c)
		defer cancel()
		body, err := reads.WorkStatus(ctx, db, id)
		writeJSON(c, body, err)
	})
	g.GET("/sources", func(c *gin.Context) {
		ctx, cancel := readContext(c)
		defer cancel()
		body, err := reads.Sources(ctx, db)
		writeJSON(c, body, err)
	})
	g.GET("/quality-runs", func(c *gin.Context) {
		n := limit(c)
		if n == 0 {
			return
		}
		ctx, cancel := readContext(c)
		defer cancel()
		body, err := reads.QualityRuns(ctx, db, n)
		writeJSON(c, body, err)
	})
	g.GET("/sync-runs", func(c *gin.Context) {
		n := limit(c)
		if n == 0 {
			return
		}
		ctx, cancel := readContext(c)
		defer cancel()
		body, err := reads.SyncRuns(ctx, db, c.Query("source"), n)
		writeJSON(c, body, err)
	})
	g.GET("/match-candidates", func(c *gin.Context) {
		n := limit(c)
		if n == 0 {
			return
		}
		ctx, cancel := readContext(c)
		defer cancel()
		body, err := reads.MatchCandidates(ctx, db, n)
		writeJSON(c, body, err)
	})
}
