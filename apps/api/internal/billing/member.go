package billing

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/smartwalle/alipay/v3"
)

var planOrderNumber = regexp.MustCompile("^MP[0-9a-f]{32}$")
var errPendingPlan = errors.New("another plan awaits payment")

// PlanRoutes is registered inside the authenticated reader group, independently of prepaid wallets.
func PlanRoutes(g *gin.RouterGroup, db *pgxpool.Pool) {
	m, _ := configuredMerchant()
	g.GET("/membership", func(c *gin.Context) {
		jsonRow(c, db, `SELECT jsonb_build_object('enabled',$2::boolean,'periodStart',period_start,'periodEnd',period_end,
 'plan',plan,'credits',credits,'usedMicro',coalesce((SELECT sum(microcredits) FROM reader_credit_usage WHERE reader_id=m.reader_id AND period_start=m.period_start),0),
 'plans',(SELECT jsonb_agg(jsonb_build_object('id',id,'fen',price_fen,'credits',credits) ORDER BY price_fen) FROM reader_plans),
 'storeOrders',(SELECT coalesce(jsonb_agg(to_jsonb(x) ORDER BY x.purchased_at DESC),'[]') FROM
 (SELECT transaction_id AS id,plan,purchased_at,currency,price_milli,revoked_at IS NOT NULL AS refunded
 FROM reader_store_purchases WHERE reader_id=m.reader_id ORDER BY purchased_at DESC LIMIT 20)x),
 'orders',(SELECT coalesce(jsonb_agg(to_jsonb(x) ORDER BY x.created_at DESC),'[]') FROM
 (SELECT id,plan,amount_fen,created_at,starts_at,ends_at,CASE WHEN refunded_at IS NOT NULL THEN 'refunded' WHEN paid_at IS NOT NULL THEN 'paid'
 WHEN expires_at<=now() OR closed THEN 'closed' ELSE 'pending' END status FROM reader_plan_orders WHERE reader_id=m.reader_id ORDER BY created_at DESC LIMIT 10)x))
 FROM reader_memberships m WHERE reader_id=$1`, c.GetString("reader"), m != nil)
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
			if err := tx.QueryRow(ctx, "SELECT id FROM reader_identities WHERE id=$1 FOR UPDATE", owner).Scan(&lock); err != nil {
				return err
			}
			var active bool
			if err := tx.QueryRow(ctx, "SELECT plan<>'free' FROM reader_memberships WHERE reader_id=$1", owner).Scan(&active); err != nil {
				return err
			}
			if active {
				return ErrConflict
			}
			var existing string
			err := tx.QueryRow(ctx, "SELECT id,plan,amount_fen,expires_at FROM reader_plan_orders WHERE reader_id=$1 AND trade_no IS NULL AND NOT closed AND expires_at>now() ORDER BY created_at DESC LIMIT 1", owner).Scan(&id, &existing, &fen, &expires)
			if err == nil {
				if existing != in.Plan || fen != in.ExpectedFen {
					return errPendingPlan
				}
				return nil
			}
			if !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
			return tx.QueryRow(ctx, `INSERT INTO reader_plan_orders(reader_id,plan,amount_fen,credits,app_id,seller_id)
 SELECT $1,id,price_fen,credits,$3,$4 FROM reader_plans WHERE id=$2 AND price_fen=$5 RETURNING id,amount_fen,expires_at`, owner, in.Plan, m.app, m.seller, in.ExpectedFen).Scan(&id, &fen, &expires)
		})
		if errors.Is(err, errPendingPlan) {
			c.JSON(409, gin.H{"error": "已有待付款订单。可继续支付原套餐，或在订单到期后另选。"})
			return
		}
		if errors.Is(err, ErrConflict) || errors.Is(err, pgx.ErrNoRows) {
			c.JSON(409, gin.H{"error": "会员已生效或套餐价格已更新，请刷新。"})
			return
		}
		if err != nil {
			fail(c, err)
			return
		}
		m.checkoutURL(c, id, expires, "MP", "Folio 时光 · "+in.Plan+" 会员（30天）", strings.TrimSuffix(m.returnURL, "/commissions")+"/me?payment=return", fen)
	})
}

func (m *alipayMerchant) applyPlanNotification(ctx context.Context, db *pgxpool.Pool, n *alipay.Notification) error {
	return pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		id := n.OutTradeNo[2:]
		var reader string
		var fen int
		var trade *string
		// Use the same lock order as checkout. Duplicate callbacks and late notifications serialize per reader.
		if err := tx.QueryRow(ctx, "SELECT r.id FROM reader_plan_orders o JOIN reader_identities r ON r.id=o.reader_id WHERE o.id=$1 AND o.app_id=$2 AND o.seller_id=$3 FOR UPDATE OF r", id, m.app, m.seller).Scan(&reader); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, "SELECT amount_fen,trade_no FROM reader_plan_orders WHERE id=$1 FOR UPDATE", id).Scan(&fen, &trade); err != nil {
			return err
		}
		if !parsesFen(n.TotalAmount, fen) || (trade != nil && *trade != n.TradeNo) {
			return ErrConflict
		}
		if n.OutBizNo != "" || n.GmtRefund != "" || n.RefundFee != "" || n.RefundAmount != "" {
			if trade == nil || n.TradeStatus != "TRADE_CLOSED" || !(parsesFen(n.RefundFee, fen) || parsesFen(n.RefundAmount, fen)) {
				return ErrConflict
			}
			_, err := tx.Exec(ctx, "UPDATE reader_plan_orders SET refunded_at=coalesce(refunded_at,now()) WHERE id=$1", id)
			return err
		}
		switch n.TradeStatus {
		case "WAIT_BUYER_PAY":
			return nil
		case "TRADE_CLOSED":
			if trade != nil {
				return ErrConflict
			}
			_, err := tx.Exec(ctx, "UPDATE reader_plan_orders SET closed=true WHERE id=$1", id)
			return err
		case "TRADE_SUCCESS", "TRADE_FINISHED":
			if trade != nil {
				return nil
			}
		default:
			return ErrConflict
		}
		paid, err := time.ParseInLocation(time.DateTime, n.GmtPayment, chinaTime)
		if err != nil {
			return ErrConflict
		}
		// If two genuine payments arrive out of order, both earn their full, non-overlapping term.
		_, err = tx.Exec(ctx, `WITH term AS (SELECT greatest($3::timestamptz,max(ends_at)) AS begins FROM reader_paid_periods WHERE reader_id=$4)
 UPDATE reader_plan_orders SET trade_no=$2,paid_at=$3,closed=false,starts_at=term.begins,ends_at=term.begins+interval '720 hours' FROM term WHERE id=$1`, id, n.TradeNo, paid, reader)
		return err
	})
}
