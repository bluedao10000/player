package billing

import (
	"context"
	"errors"
	"regexp"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/smartwalle/alipay/v3"

	queries "anime-agent/api/internal/repository/queries/billing"
	"anime-agent/api/internal/repository/queries/reader"
	domain "anime-agent/api/internal/service/billing"
)

var planOrderNumber = regexp.MustCompile("^MP[0-9a-f]{32}$")
var errPendingPlan = errors.New("another plan awaits payment")

// PlanRoutes is registered inside the authenticated reader group.
func PlanRoutes(g *gin.RouterGroup, db *pgxpool.Pool) {
	m, _ := configuredMerchant()
	g.GET("/membership", func(c *gin.Context) {
		jsonRow(c, db, queries.PlanSummary, c.GetString("reader"), m != nil)
	})
	g.POST("/membership/checkout", func(c *gin.Context) {
		if m == nil {
			c.JSON(503, gin.H{"error": "支付宝收款暂不可用。"})
			return
		}
		var in struct {
			Plan        string
			ExpectedFen int
		}
		if !input(c, &in) {
			return
		}
		if in.Plan != "plus" && in.Plan != "pro" {
			c.JSON(400, gin.H{"error": "请选择 Plus 或 Pro。"})
			return
		}
		ctx := c.Request.Context()
		owner := c.GetString("reader")
		var id string
		var fen int
		var expires time.Time
		err := pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
			var lock string
			if err := tx.QueryRow(ctx, reader.LockReader, owner).Scan(&lock); err != nil {
				return err
			}
			var active bool
			if err := tx.QueryRow(ctx, queries.IsPaidMember, owner).Scan(&active); err != nil {
				return err
			}
			if active {
				return domain.ErrConflict
			}
			var existing string
			err := tx.QueryRow(ctx, queries.PendingPlanOrder, owner).Scan(&id, &existing, &fen, &expires)
			if err == nil {
				if existing != in.Plan || fen != in.ExpectedFen {
					return errPendingPlan
				}
				return nil
			}
			if !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
			return tx.QueryRow(ctx, queries.CreatePlanOrder, owner, in.Plan, m.app, m.seller, in.ExpectedFen).Scan(&id, &fen, &expires)
		})
		if errors.Is(err, errPendingPlan) {
			c.JSON(409, gin.H{"error": "已有待付款订单。可继续支付原套餐，或在订单到期后另选。"})
			return
		}
		if errors.Is(err, domain.ErrConflict) || errors.Is(err, pgx.ErrNoRows) {
			c.JSON(409, gin.H{"error": "会员已生效或套餐价格已更新，请刷新。"})
			return
		}
		if err != nil {
			fail(c, err)
			return
		}
		m.checkoutURL(c, id, expires, "Folio 时光 · "+in.Plan+" 会员（30天）", fen)
	})
}

func (m *alipayMerchant) applyPlanNotification(ctx context.Context, db *pgxpool.Pool, n *alipay.Notification) error {
	return pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		id := n.OutTradeNo[2:]
		var reader string
		var fen int
		var trade *string
		// Use the same lock order as checkout. Duplicate callbacks and late notifications serialize per reader.
		if err := tx.QueryRow(ctx, queries.LockPlanOrderReader, id, m.app, m.seller).Scan(&reader); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, queries.LockPlanPayment, id).Scan(&fen, &trade); err != nil {
			return err
		}
		if !parsesFen(n.TotalAmount, fen) || (trade != nil && *trade != n.TradeNo) {
			return domain.ErrConflict
		}
		if n.OutBizNo != "" || n.GmtRefund != "" || n.RefundFee != "" || n.RefundAmount != "" {
			if trade == nil || n.TradeStatus != "TRADE_CLOSED" || !(parsesFen(n.RefundFee, fen) || parsesFen(n.RefundAmount, fen)) {
				return domain.ErrConflict
			}
			_, err := tx.Exec(ctx, queries.RefundPlanOrder, id)
			return err
		}
		switch n.TradeStatus {
		case "WAIT_BUYER_PAY":
			return nil
		case "TRADE_CLOSED":
			if trade != nil {
				return domain.ErrConflict
			}
			_, err := tx.Exec(ctx, queries.ClosePlanOrder, id)
			return err
		case "TRADE_SUCCESS", "TRADE_FINISHED":
			if trade != nil {
				return nil
			}
		default:
			return domain.ErrConflict
		}
		paid, err := time.ParseInLocation(time.DateTime, n.GmtPayment, chinaTime)
		if err != nil {
			return domain.ErrConflict
		}
		// If two genuine payments arrive out of order, both earn their full, non-overlapping term.
		_, err = tx.Exec(ctx, queries.ActivatePaidPeriod, id, n.TradeNo, paid, reader)
		return err
	})
}
