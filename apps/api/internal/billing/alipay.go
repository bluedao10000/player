package billing

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/smartwalle/alipay/v3"
)

const notifyPath = "/api/billing/alipay/notify"

var chinaTime = time.FixedZone("UTC+8", 8*3600)

type wapMerchant struct {
	client                         *alipay.Client
	app, seller, notify, returnURL string
}

func configuredMerchant() (*wapMerchant, error) {
	env := func(k string) string { return strings.TrimSpace(os.Getenv("ALIPAY_" + k)) }
	app, seller, notify, private, public := env("APP_ID"), env("SELLER_ID"), env("NOTIFY_URL"), env("PRIVATE_KEY"), env("PUBLIC_KEY")
	if app+seller+notify+private+public == "" {
		return nil, nil
	}
	u, err := url.Parse(notify)
	if app == "" || seller == "" || private == "" || public == "" || err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Path == "" || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("check the five ALIPAY settings and public HTTPS notification path")
	}
	client, err := alipay.New(app, strings.ReplaceAll(private, `\n`, "\n"), true, alipay.WithTimeLocation(chinaTime))
	if err != nil {
		return nil, errors.New("invalid ALIPAY_PRIVATE_KEY")
	}
	if err = client.LoadAliPayPublicKey(strings.ReplaceAll(public, `\n`, "\n")); err != nil {
		return nil, errors.New("invalid ALIPAY_PUBLIC_KEY")
	}
	return &wapMerchant{client, app, seller, notify, u.Scheme + "://" + u.Host + "/commissions"}, nil
}

func fenText(fen int) string { return fmt.Sprintf("%d.%02d", fen/100, fen%100) }

var yuanPattern = regexp.MustCompile(`^([0-9]{1,8})(?:\.([0-9]{1,2}))?$`)

func parsesFen(raw string, want int) bool {
	digits := yuanPattern.FindStringSubmatch(raw)
	if digits == nil {
		return false
	}
	yuan, _ := strconv.Atoi(digits[1])
	frac, _ := strconv.Atoi((digits[2] + "00")[:2])
	return yuan*100+frac == want
}

func (m *wapMerchant) checkout(c *gin.Context, db *pgxpool.Pool) {
	var v struct {
		Key string `json:"client_key" binding:"required,uuid"`
		Fen int    `json:"amount_fen" binding:"min=1,max=1000000"`
	}
	if !input(c, &v) {
		return
	}
	var id string
	var expires time.Time
	var payable bool
	err := db.QueryRow(c.Request.Context(), `INSERT INTO billing_alipay_topups(account_id,client_key,amount_fen,app_id,seller_id)
 SELECT id,$2,$3,$4,$5 FROM billing_accounts WHERE id=$1 AND currency='CNY'
 ON CONFLICT(account_id,client_key) DO UPDATE SET client_key=excluded.client_key
 WHERE (billing_alipay_topups.amount_fen,billing_alipay_topups.app_id,billing_alipay_topups.seller_id)=(excluded.amount_fen,excluded.app_id,excluded.seller_id)
 RETURNING id::text,expires_at,paid_at IS NULL AND NOT closed AND expires_at>now()`, c.GetString("billing_account"), v.Key, v.Fen, m.app, m.seller).Scan(&id, &expires, &payable)
	if errors.Is(err, pgx.ErrNoRows) {
		err = ErrConflict
	}
	if err != nil {
		fail(c, err)
		return
	}
	if !payable {
		c.JSON(409, gin.H{"error": "checkout is no longer payable"})
		return
	}
	m.wapURL(c, id, expires, "TP", "Folio 时光 · 读者余额充值", m.returnURL, v.Fen)
}

func (m *wapMerchant) wapURL(c *gin.Context, id string, expires time.Time, prefix, subject, returnURL string, fen int) {
	p := alipay.TradeWapPay{}
	p.NotifyURL = m.notify
	p.ReturnURL = returnURL
	p.QuitURL = returnURL
	p.Subject = subject
	p.ProductCode = "QUICK_WAP_WAY"
	p.OutTradeNo = prefix + strings.ReplaceAll(id, "-", "")
	p.TotalAmount = fenText(fen)
	p.SellerId = m.seller
	p.TimeExpire = expires.In(chinaTime).Format(time.DateTime)
	redirect, err := m.client.TradeWapPay(p)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, gin.H{"id": id, "checkout_url": redirect.String()})
}

var merchantOrder = regexp.MustCompile(`^TP[0-9a-f]{32}$`)
var tradeNumber = regexp.MustCompile(`^[0-9]{1,64}$`)

// Payment confirmation and the prepaid credit commit together. A browser return never calls this function.
func (m *wapMerchant) notifyPayment(ctx context.Context, db *pgxpool.Pool, values url.Values) error {
	for _, v := range values {
		if len(v) != 1 {
			return ErrConflict
		}
	}
	if values.Get("sign_type") != "RSA2" || values.Has("alipay_cert_sn") || values.Get("notify_id") == "" || values.Get("notify_type") != "trade_status_sync" || values.Get("app_id") != m.app || values.Get("seller_id") != m.seller {
		return ErrConflict
	}
	n, err := m.client.DecodeNotification(ctx, values)
	if err != nil {
		return ErrConflict
	}
	membership := planOrderNumber.MatchString(n.OutTradeNo)
	if (!membership && !merchantOrder.MatchString(n.OutTradeNo)) || !tradeNumber.MatchString(n.TradeNo) {
		return ErrConflict
	}
	if membership {
		return m.applyPlanNotification(ctx, db, n)
	}
	if n.OutBizNo != "" || n.GmtRefund != "" || n.RefundFee != "" || n.RefundAmount != "" {
		return errors.New("Alipay refund notification requires channel reconciliation")
	}
	return pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		var owner string
		var fen int
		var trade *string
		err := tx.QueryRow(ctx, `SELECT account_id::text,amount_fen,trade_no FROM billing_alipay_topups
 WHERE id=$1 AND app_id=$2 AND seller_id=$3 FOR UPDATE`, n.OutTradeNo[2:], m.app, m.seller).Scan(&owner, &fen, &trade)
		if err != nil {
			return err
		}
		if !parsesFen(n.TotalAmount, fen) || trade != nil && *trade != n.TradeNo {
			return ErrConflict
		}
		switch n.TradeStatus {
		case "WAIT_BUYER_PAY":
			return nil
		case "TRADE_CLOSED":
			if trade != nil {
				return errors.New("closed paid topup requires channel reconciliation")
			}
			_, err = tx.Exec(ctx, "UPDATE billing_alipay_topups SET closed=true WHERE id=$1", n.OutTradeNo[2:])
			return err
		case "TRADE_SUCCESS", "TRADE_FINISHED":
			if trade != nil {
				return nil
			}
		default:
			return ErrConflict
		}
		if err = accountLock(ctx, tx, owner); err != nil {
			return err
		}
		var entry int64
		err = tx.QueryRow(ctx, `INSERT INTO billing_entries(account_id,kind,reference,amount_micros,actor)
 VALUES($1,'credit',$2,$3,'alipay-notify') RETURNING id`, owner, "alipay:"+n.TradeNo, int64(fen)*10000).Scan(&entry)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE billing_alipay_topups SET trade_no=$2,entry_id=$3,paid_at=now(),closed=false WHERE id=$1`, n.OutTradeNo[2:], n.TradeNo, entry)
		return err
	})
}

func alipayRoutes(r *gin.Engine, g *gin.RouterGroup, db *pgxpool.Pool) bool {
	m, err := configuredMerchant()
	if err != nil {
		log.Printf("Alipay disabled: %v", err)
	}
	g.POST("/alipay/topups", func(c *gin.Context) {
		if m == nil {
			c.Status(503)
			return
		}
		m.checkout(c, db)
	})
	g.GET("/alipay/topups", func(c *gin.Context) {
		jsonRow(c, db, `SELECT coalesce(jsonb_agg(to_jsonb(x) ORDER BY created_at DESC),'[]') FROM
 (SELECT id,client_key,amount_fen,created_at,CASE WHEN paid_at IS NOT NULL THEN 'paid' WHEN closed OR expires_at<=now() THEN 'closed' ELSE 'pending' END status
 FROM billing_alipay_topups WHERE account_id=$1 ORDER BY created_at DESC LIMIT 20)x`, c.GetString("billing_account"))
	})
	r.POST(notifyPath, func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		if m == nil {
			c.String(503, "failure")
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16384)
		if c.ContentType() != "application/x-www-form-urlencoded" || c.Request.ParseForm() != nil {
			c.String(400, "failure")
			return
		}
		if err := m.notifyPayment(c.Request.Context(), db, c.Request.PostForm); err != nil {
			if !errors.Is(err, ErrConflict) && !errors.Is(err, pgx.ErrNoRows) {
				log.Printf("Alipay notification not committed: %v", err)
			}
			c.String(400, "failure")
			return
		}
		c.String(200, "success")
	})
	return m != nil
}
