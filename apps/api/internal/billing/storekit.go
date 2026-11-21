package billing

import (
	"context"
	"crypto/ecdsa"
	"crypto/x509"
	_ "embed"
	"encoding/base64"
	"errors"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed applecert/AppleRootCA-G3.cer
var appleRoot []byte

const storeBundle = "com.kmszz.starWiki"

var storePlans = map[string]string{storeBundle + ".plus.30days": "plus", storeBundle + ".pro.30days": "pro"}
var uuidText = regexp.MustCompile("^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$")

type storeTransaction struct {
	jwt.RegisteredClaims
	TransactionID, BundleID, ProductID, AppAccountToken, Environment, Type, Currency string
	PurchaseDate, SignedDate, RevocationDate, Price                                  int64
	Quantity                                                                         int
}
type storeNotification struct {
	jwt.RegisteredClaims
	NotificationType string
	SignedDate       int64
	Data             struct {
		SignedTransactionInfo, BundleID, Environment string
		AppAppleID                                   int64
	}
}

func hasMarker(cert *x509.Certificate, oid string) bool {
	for _, v := range cert.Extensions {
		if v.Id.String() == oid {
			return true
		}
	}
	return false
}

// Verify the payload with Apple's App Store leaf and WWDR intermediate markers.
// Historical purchases can be restored after the signing certificate expires.
func verifyStoreJWS(raw string, claims jwt.Claims, date *int64) error {
	if len(raw) > 64000 {
		return ErrConflict
	}
	_, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (any, error) {
		chain, ok := t.Header["x5c"].([]any)
		if !ok || len(chain) != 3 || *date <= 0 || time.UnixMilli(*date).After(time.Now().Add(2*time.Minute)) {
			return nil, ErrConflict
		}
		certs := make([]*x509.Certificate, 3)
		for i, v := range chain {
			s, ok := v.(string)
			if !ok {
				return nil, ErrConflict
			}
			der, e := base64.StdEncoding.DecodeString(s)
			if e != nil {
				return nil, e
			}
			certs[i], e = x509.ParseCertificate(der)
			if e != nil {
				return nil, e
			}
		}
		root, e := x509.ParseCertificate(appleRoot)
		if e != nil {
			return nil, e
		}
		if !root.Equal(certs[2]) || !hasMarker(certs[0], "1.2.840.113635.100.6.11.1") || !hasMarker(certs[1], "1.2.840.113635.100.6.2.1") {
			return nil, ErrConflict
		}
		roots, inter := x509.NewCertPool(), x509.NewCertPool()
		roots.AddCert(root)
		inter.AddCert(certs[1])
		chains, e := certs[0].Verify(x509.VerifyOptions{Roots: roots, Intermediates: inter, CurrentTime: time.UnixMilli(*date), KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageAny}})
		if e != nil {
			return nil, e
		}
		for _, ch := range chains {
			if len(ch) == 3 && ch[1].Equal(certs[1]) {
				key, ok := certs[0].PublicKey.(*ecdsa.PublicKey)
				if ok {
					return key, nil
				}
			}
		}
		return nil, ErrConflict
	}, jwt.WithValidMethods([]string{"ES256"}))
	return err
}
func allowedStore(bundle, env string) bool {
	if os.Getenv("STOREKIT_ENVIRONMENT") == "Sandbox" {
		return env == "Sandbox" && (bundle == storeBundle || bundle == storeBundle+".test" || bundle == storeBundle+".dev")
	}
	return env == "Production" && bundle == storeBundle
}
func (t *storeTransaction) validEnvelope() bool {
	return allowedStore(t.BundleID, t.Environment) && tradeNumber.MatchString(t.TransactionID) && uuidText.MatchString(t.AppAccountToken) && t.Quantity == 1 && t.PurchaseDate > 0 && t.PurchaseDate <= t.SignedDate && t.Price >= 0
}
func recordStore(ctx context.Context, db *pgxpool.Pool, t storeTransaction, owner string, reverse bool) error {
	if !t.validEnvelope() || storePlans[t.ProductID] == "" || t.Type != "Non-Renewing Subscription" || !strings.EqualFold(owner, t.AppAccountToken) {
		return ErrConflict
	}
	return pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		var id string
		if e := tx.QueryRow(ctx, "SELECT id FROM reader_identities WHERE id=$1 FOR UPDATE", owner).Scan(&id); e != nil {
			return e
		}
		var revoked *time.Time
		if t.RevocationDate > 0 {
			d := time.UnixMilli(t.RevocationDate)
			revoked = &d
		}
		tag, e := tx.Exec(ctx, `WITH occupied AS (
 SELECT range_agg(tstzrange(starts_at,ends_at,'[)')) periods FROM reader_paid_periods WHERE reader_id=$2 AND ends_at>$7
), term AS (
 SELECT greatest($7::timestamptz,max(upper(period))) begins FROM occupied,unnest(periods) period WHERE period @> $7::timestamptz
)
 INSERT INTO reader_store_purchases(transaction_id,reader_id,environment,bundle_id,product_id,plan,credits,purchased_at,starts_at,ends_at,revoked_at,signed_at,currency,price_milli,refund_event_at)
 SELECT $1,$2,$3,$4,$5,p.id,p.credits,$7,term.begins,term.begins+interval '720 hours',$8,$9,$10,$11,CASE WHEN $8::timestamptz IS NOT NULL OR $12 THEN $9::timestamptz END FROM reader_plans p,term WHERE p.id=$6
 ON CONFLICT(transaction_id) DO UPDATE SET
 revoked_at=CASE WHEN excluded.refund_event_at IS NOT NULL AND (reader_store_purchases.refund_event_at IS NULL OR excluded.refund_event_at>=reader_store_purchases.refund_event_at)
 THEN excluded.revoked_at ELSE reader_store_purchases.revoked_at END,
 refund_event_at=greatest(excluded.refund_event_at,reader_store_purchases.refund_event_at),
 signed_at=greatest(excluded.signed_at,reader_store_purchases.signed_at)
 WHERE (reader_store_purchases.reader_id,reader_store_purchases.bundle_id,reader_store_purchases.environment,reader_store_purchases.product_id)
 =(excluded.reader_id,excluded.bundle_id,excluded.environment,excluded.product_id)`, t.TransactionID, owner, t.Environment, t.BundleID, t.ProductID, storePlans[t.ProductID], time.UnixMilli(t.PurchaseDate), revoked, time.UnixMilli(t.SignedDate), t.Currency, t.Price, reverse)
		if e == nil && tag.RowsAffected() != 1 {
			return ErrConflict
		}
		return e
	})
}
func storeReply(c *gin.Context, err error) {
	if err == nil {
		c.Status(204)
		return
	}
	if errors.Is(err, ErrConflict) || errors.Is(err, pgx.ErrNoRows) {
		c.JSON(409, gin.H{"error": "交易与当前读者或商品不匹配。"})
		return
	}
	fail(c, err)
}
func StoreReceipt(db *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		var in struct{ SignedTransaction string }
		var t storeTransaction
		if c.ShouldBindJSON(&in) != nil || verifyStoreJWS(in.SignedTransaction, &t, &t.SignedDate) != nil {
			c.JSON(400, gin.H{"error": "App Store 交易验签失败。"})
			return
		}
		storeReply(c, recordStore(c.Request.Context(), db, t, c.GetString("reader"), false))
	}
}
func StoreNotifications(db *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		var in struct{ SignedPayload string }
		var n storeNotification
		if c.ShouldBindJSON(&in) != nil || verifyStoreJWS(in.SignedPayload, &n, &n.SignedDate) != nil || !allowedStore(n.Data.BundleID, n.Data.Environment) || (n.Data.Environment == "Production" && n.Data.AppAppleID != 1496915312) {
			c.Status(400)
			return
		}
		switch n.NotificationType {
		case "REFUND", "REVOKE", "REFUND_REVERSED", "ONE_TIME_CHARGE":
		default:
			c.Status(204)
			return
		}
		var t storeTransaction
		if verifyStoreJWS(n.Data.SignedTransactionInfo, &t, &t.SignedDate) != nil || t.BundleID != n.Data.BundleID || t.Environment != n.Data.Environment {
			c.Status(400)
			return
		}
		if storePlans[t.ProductID] == "" && t.ProductID != walletProduct {
			c.Status(204)
			return
		}
		if (n.NotificationType == "REFUND" || n.NotificationType == "REVOKE") && t.RevocationDate == 0 {
			c.Status(400)
			return
		}
		reverse := n.NotificationType == "REFUND_REVERSED"
		if reverse && t.RevocationDate != 0 {
			c.Status(400)
			return
		}
		if reverse || n.NotificationType == "REFUND" || n.NotificationType == "REVOKE" {
			t.SignedDate = n.SignedDate
		}
		if t.ProductID == walletProduct {
			storeReply(c, recordWallet(c.Request.Context(), db, t, reverse))
			return
		}
		storeReply(c, recordStore(c.Request.Context(), db, t, t.AppAccountToken, reverse))
	}
}
