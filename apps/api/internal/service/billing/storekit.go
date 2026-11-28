package billing

import (
	"context"
	"crypto/ecdsa"
	"crypto/x509"
	_ "embed"
	"encoding/base64"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	queries "anime-agent/api/internal/repository/queries/billing"
	"anime-agent/api/internal/repository/queries/reader"
)

//go:embed applecert/AppleRootCA-G3.cer
var appleRoot []byte

const storeBundle = "com.kmszz.starWiki"

var storePlans = map[string]string{storeBundle + ".plus.30days": "plus", storeBundle + ".pro.30days": "pro"}
var tradeNumber = regexp.MustCompile(`^[0-9]{1,64}$`)
var uuidText = regexp.MustCompile("^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$")

type StoreTransaction struct {
	jwt.RegisteredClaims
	TransactionID, BundleID, ProductID, AppAccountToken, Environment, Type, Currency string
	PurchaseDate, SignedDate, RevocationDate, Price                                  int64
	Quantity                                                                         int
}
type StoreNotification struct {
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
func VerifyStoreJWS(raw string, claims jwt.Claims, date *int64) error {
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
func AllowedStore(bundle, env string) bool {
	if os.Getenv("STOREKIT_ENVIRONMENT") == "Sandbox" {
		return env == "Sandbox" && (bundle == storeBundle || bundle == storeBundle+".test" || bundle == storeBundle+".dev")
	}
	return env == "Production" && bundle == storeBundle
}
func (t *StoreTransaction) validEnvelope() bool {
	return AllowedStore(t.BundleID, t.Environment) && tradeNumber.MatchString(t.TransactionID) && uuidText.MatchString(t.AppAccountToken) && t.Quantity == 1 && t.PurchaseDate > 0 && t.PurchaseDate <= t.SignedDate && t.Price >= 0
}
func RecordStore(ctx context.Context, db *pgxpool.Pool, t StoreTransaction, owner string, reverse bool) error {
	if !t.validEnvelope() || storePlans[t.ProductID] == "" || t.Type != "Non-Renewing Subscription" || !strings.EqualFold(owner, t.AppAccountToken) {
		return ErrConflict
	}
	return pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		var id string
		if e := tx.QueryRow(ctx, reader.LockReader, owner).Scan(&id); e != nil {
			return e
		}
		var revoked *time.Time
		if t.RevocationDate > 0 {
			d := time.UnixMilli(t.RevocationDate)
			revoked = &d
		}
		tag, e := tx.Exec(ctx, queries.RecordStorePurchase, t.TransactionID, owner, t.Environment, t.BundleID, t.ProductID, storePlans[t.ProductID], time.UnixMilli(t.PurchaseDate), revoked, time.UnixMilli(t.SignedDate), t.Currency, t.Price, reverse)
		if e == nil && tag.RowsAffected() != 1 {
			return ErrConflict
		}
		return e
	})
}

func IsStorePlan(id string) bool { return storePlans[id] != "" }
