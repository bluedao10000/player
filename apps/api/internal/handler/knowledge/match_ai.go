package knowledge

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"anime-agent/api/internal/repository/administration"
	"anime-agent/api/internal/service/matching"
)

func matchAIRoutes(g *gin.RouterGroup, db *pgxpool.Pool) {
	g.GET("/matches/:id/assessments", func(c *gin.Context) {
		ctx, cancel := readContext(c)
		defer cancel()
		body, err := administration.Assessments(ctx, db, c.Param("id"))
		writeJSON(c, body, err)
	})
	g.POST("/matches/:id/assessments", func(c *gin.Context) {
		id, e := strconv.ParseInt(c.Param("id"), 10, 64)
		if e != nil || id <= 0 {
			c.Status(400)
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 45*time.Second)
		defer cancel()
		result, err := matching.Assess(ctx, db, id, c.GetString("operator"))
		if err != nil {
			var upstream *matching.ProviderError
			status := 500
			switch {
			case errors.Is(err, pgx.ErrNoRows):
				status = 404
			case errors.Is(err, matching.ErrBusy), errors.Is(err, matching.ErrChanged):
				status = 409
			case errors.Is(err, matching.ErrBudget):
				status = 422
			case errors.As(err, &upstream):
				status = 502
				if upstream.Configuration {
					status = 503
				}
			}
			if status == 500 {
				c.Status(status)
			} else {
				c.JSON(status, gin.H{"error": err.Error()})
			}
			return
		}
		c.Data(200, "application/json", result)
	})
}
