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

	domain "anime-agent/api/internal/service/billing"
)

const notifyPath = "/api/billing/alipay/notify"

var chinaTime = time.FixedZone("UTC+8", 8*3600)

type alipayMerchant struct {
	client                         *alipay.Client
	app, seller, notify, returnURL string
}

func configuredMerchant() (*alipayMerchant, error) {
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
	return &alipayMerchant{client, app, seller, notify, u.Scheme + "://" + u.Host + "/me?payment=return"}, nil
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

func (m *alipayMerchant) checkoutURL(c *gin.Context, id string, expires time.Time, subject string, fen int) {
	p := alipay.TradePagePay{}
	p.NotifyURL = m.notify
	p.ReturnURL = m.returnURL
	p.Subject = subject
	p.ProductCode = "FAST_INSTANT_TRADE_PAY"
	p.OutTradeNo = "MP" + strings.ReplaceAll(id, "-", "")
	p.TotalAmount = fenText(fen)
	p.SellerId = m.seller
	p.TimeExpire = expires.In(chinaTime).Format(time.DateTime)
	redirect, err := m.client.TradePagePay(p)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(200, gin.H{"id": id, "checkout_url": redirect.String()})
}

var tradeNumber = regexp.MustCompile(`^[0-9]{1,64}$`)

// Only a signed provider notification can activate a membership.
func (m *alipayMerchant) notifyPayment(ctx context.Context, db *pgxpool.Pool, values url.Values) error {
	for _, v := range values {
		if len(v) != 1 {
			return domain.ErrConflict
		}
	}
	if values.Get("sign_type") != "RSA2" || values.Has("alipay_cert_sn") || values.Get("notify_id") == "" || values.Get("notify_type") != "trade_status_sync" || values.Get("app_id") != m.app || values.Get("seller_id") != m.seller {
		return domain.ErrConflict
	}
	n, err := m.client.DecodeNotification(ctx, values)
	if err != nil {
		return domain.ErrConflict
	}
	if !planOrderNumber.MatchString(n.OutTradeNo) || !tradeNumber.MatchString(n.TradeNo) {
		return domain.ErrConflict
	}
	return m.applyPlanNotification(ctx, db, n)
}

func PaymentRoutes(r *gin.Engine, db *pgxpool.Pool) {
	m, err := configuredMerchant()
	if err != nil {
		log.Printf("Alipay disabled: %v", err)
	}
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
			if !errors.Is(err, domain.ErrConflict) && !errors.Is(err, pgx.ErrNoRows) {
				log.Printf("Alipay notification not committed: %v", err)
			}
			c.String(400, "failure")
			return
		}
		c.String(200, "success")
	})
}
