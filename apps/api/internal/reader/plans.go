package reader

import (
	"errors"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
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
			err := tx.QueryRow(ctx, "SELECT id FROM reader_identities WHERE id=$1 FOR UPDATE", key).Scan(&id)
			if err != nil {
				return err
			}
			result, err := tx.Exec(ctx, "UPDATE reader_identities SET plan=$2,plan_until=$3 WHERE id=$1 AND (plan,plan_until) IS DISTINCT FROM ($2::text,$3::timestamptz)", id, in.Plan, in.ValidUntil)
			if err != nil || result.RowsAffected() == 0 {
				return err
			}
			_, err = tx.Exec(ctx, "INSERT INTO reader_plan_changes(reader_id,plan,valid_until,operator) VALUES($1,$2,$3,$4)", id, in.Plan, in.ValidUntil, c.GetString("operator"))
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
