package knowledge

import (
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	queries "anime-agent/api/internal/repository/queries/knowledge"
)

func sourceReviewRoutes(g *gin.RouterGroup, db *pgxpool.Pool) {
	g.POST("/source-reviews/:id/resolve", func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "核对记录编号无效。"})
			return
		}
		var input struct {
			LastRunID string `json:"last_run_id" binding:"required,uuid"`
		}
		if !editorRequest(c, &input) {
			return
		}
		ctx, cancel := readContext(c)
		defer cancel()
		result, err := db.Exec(ctx, queries.ResolveSourceReview, id, input.LastRunID, c.GetString("operator"))
		if err != nil {
			log.Printf("resolve source review: %v", err)
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "暂时无法保存核对结果。请重试。"})
			return
		}
		if result.RowsAffected() == 0 {
			c.JSON(http.StatusConflict, gin.H{"error": "核对记录已变更。请刷新后再核对。"})
			return
		}
		c.Status(http.StatusNoContent)
	})
}
