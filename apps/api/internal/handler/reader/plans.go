package reader

import (
	"errors"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	queries "anime-agent/database/queries/reader"
)

func AdminRoutes(g *gin.RouterGroup, db *pgxpool.Pool) {
	g.PUT("/readers/:id/plan", func(c *gin.Context) {
		var key pgtype.UUID
		var in struct {
			Plan       string
			ValidUntil *time.Time `json:"validUntil"`
		}
		if key.Scan(c.Param("id")) != nil || c.ShouldBindJSON(&in) != nil || (in.Plan != "free" && in.Plan != "plus" && in.Plan != "pro") || (in.ValidUntil != nil && (in.Plan == "free" || !in.ValidUntil.After(time.Now()))) {
			fail(c, 400, "会员等级或有效期无效。")
			return
		}
		ctx := c.Request.Context()
		err := pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
			var id string
			err := tx.QueryRow(ctx, queries.LockReader, key).Scan(&id)
			if err != nil {
				return err
			}
			result, err := tx.Exec(ctx, queries.SetReaderPlan, id, in.Plan, in.ValidUntil)
			if err != nil || result.RowsAffected() == 0 {
				return err
			}
			_, err = tx.Exec(ctx, queries.RecordPlanChange, id, in.Plan, in.ValidUntil, c.GetString("operator"))
			return err
		})
		if errors.Is(err, pgx.ErrNoRows) {
			fail(c, 404, "读者不存在。")
			return
		}
		if err != nil {
			unavailable(c)
			return
		}
		c.Status(204)
	})
}
