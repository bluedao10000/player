package knowledge

import (
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	catalog "anime-agent/api/internal/model/knowledge"
	"anime-agent/api/internal/repository/workbench"
	domain "anime-agent/api/internal/service/knowledge"
)

func AdminRoutes(r *gin.Engine, g *gin.RouterGroup, db *pgxpool.Pool) {
	editorRoutes(r, g, db)
	matchAIRoutes(g, db)
	usageRoutes(g, db)
	g.POST("/quality", func(c *gin.Context) {
		work := c.Query("work_id")
		if work != "" && !catalog.ValidUUID(work) {
			c.Status(400)
			return
		}
		report, err := domain.Quality(c.Request.Context(), db, c.DefaultQuery("source", "all"), work)
		if err != nil {
			log.Print(err)
			c.JSON(422, gin.H{"error": "quality scan failed or empty scope"})
			return
		}
		c.Data(200, "application/json", report)
	})
	g.GET("/versions", func(c *gin.Context) {
		ctx, cancel := readContext(c)
		defer cancel()
		body, err := workbench.Versions(ctx, db, c.Query("source"))
		writeJSON(c, body, err)
	})
	g.GET("/matches", func(c *gin.Context) {
		n := limit(c)
		if n > 0 {
			ctx, cancel := readContext(c)
			defer cancel()
			body, err := workbench.Matches(ctx, db, n)
			writeJSON(c, body, err)
		}
	})
	g.GET("/matches/:id", func(c *gin.Context) {
		ctx, cancel := readContext(c)
		defer cancel()
		body, err := workbench.Match(ctx, db, c.Param("id"))
		writeJSON(c, body, err)
	})
	g.GET("/works/:id/issues", func(c *gin.Context) {
		ctx, cancel := readContext(c)
		defer cancel()
		body, err := workbench.Issues(ctx, db, c.Param("id"))
		writeJSON(c, body, err)
	})
	g.GET("/works/:id/history", func(c *gin.Context) {
		ctx, cancel := readContext(c)
		defer cancel()
		body, err := workbench.History(ctx, db, c.Param("id"))
		writeJSON(c, body, err)
	})
	g.POST("/actions", func(c *gin.Context) {
		var d catalog.Decision
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 32768)
		if c.ShouldBindJSON(&d) != nil {
			c.Status(400)
			return
		}
		if e := domain.Decide(c.Request.Context(), db, d, c.GetString("operator"), "admin-api"); e != nil {
			if errors.Is(e, pgx.ErrNoRows) {
				c.Status(404)
			} else {
				log.Print(e)
				c.JSON(409, gin.H{"error": "invalid or conflicting decision"})
			}
			return
		}
		c.Status(204)
	})
}
