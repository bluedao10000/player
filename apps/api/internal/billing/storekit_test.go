package billing

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/base64"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAppStoreSigningChain(t *testing.T) {
	old := appleRoot
	t.Cleanup(func() { appleRoot = old })
	makeCert := func(n int64, parent *x509.Certificate, issuer *ecdsa.PrivateKey, oid asn1.ObjectIdentifier) (*x509.Certificate, *ecdsa.PrivateKey) {
		k, e := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if e != nil {
			t.Fatal(e)
		}
		c := &x509.Certificate{SerialNumber: big.NewInt(n), Subject: pkix.Name{CommonName: fmt.Sprint(n)}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), BasicConstraintsValid: true, IsCA: n < 3, KeyUsage: x509.KeyUsageDigitalSignature}
		if c.IsCA {
			c.KeyUsage |= x509.KeyUsageCertSign
		}
		if oid != nil {
			c.ExtraExtensions = []pkix.Extension{{Id: oid, Value: []byte{5, 0}}}
		}
		if parent == nil {
			parent = c
			issuer = k
		}
		der, e := x509.CreateCertificate(rand.Reader, c, parent, &k.PublicKey, issuer)
		if e != nil {
			t.Fatal(e)
		}
		cert, e := x509.ParseCertificate(der)
		if e != nil {
			t.Fatal(e)
		}
		return cert, k
	}
	root, rk := makeCert(1, nil, nil, nil)
	inter, ik := makeCert(2, root, rk, asn1.ObjectIdentifier{1, 2, 840, 113635, 100, 6, 2, 1})
	check := func(marker asn1.ObjectIdentifier, want bool) {
		cert, key := makeCert(3, inter, ik, marker)
		tok := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{"signedDate": time.Now().UnixMilli(), "productId": storeBundle + ".plus.30days"})
		tok.Header["x5c"] = []string{base64.StdEncoding.EncodeToString(cert.Raw), base64.StdEncoding.EncodeToString(inter.Raw), base64.StdEncoding.EncodeToString(root.Raw)}
		signed, _ := tok.SignedString(key)
		var tx storeTransaction
		err := verifyStoreJWS(signed, &tx, &tx.SignedDate)
		if (err == nil) != want {
			t.Fatal("chain verification", err)
		}
	}
	marker := asn1.ObjectIdentifier{1, 2, 840, 113635, 100, 6, 11, 1}
	check(marker, false)
	appleRoot = root.Raw
	check(nil, false)
	check(marker, true)
}

func storeTestDatabase(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("KNOWLEDGE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("requires isolated PostgreSQL")
	}
	ctx := context.Background()
	admin, e := pgxpool.New(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(admin.Close)
	schema := fmt.Sprintf("ios_receipt_%d", time.Now().UnixNano())
	if _, e = admin.Exec(ctx, "CREATE SCHEMA "+schema); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _, _ = admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE") })
	cfg, e := pgxpool.ParseConfig(dsn)
	if e != nil {
		t.Fatal(e)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	db, e := pgxpool.NewWithConfig(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(db.Close)
	scripts, e := filepath.Glob("../knowledge/sql/*.sql")
	if e != nil {
		t.Fatal(e)
	}
	for _, path := range scripts {
		sql, e := os.ReadFile(path)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = db.Exec(ctx, string(sql)); e != nil {
			t.Fatal(path, e)
		}
	}
	return db
}

func TestStoreReceiptPeriods(t *testing.T) {
	db := storeTestDatabase(t)
	ctx := context.Background()
	var owner string
	e := db.QueryRow(ctx, "INSERT INTO reader_identities DEFAULT VALUES RETURNING id").Scan(&owner)
	if e != nil {
		t.Fatal(e)
	}
	t.Setenv("STOREKIT_ENVIRONMENT", "Production")
	now := time.Now().Add(-time.Hour)
	receipt := storeTransaction{TransactionID: "123450", BundleID: storeBundle, ProductID: storeBundle + ".plus.30days", Type: "Non-Renewing Subscription", AppAccountToken: owner, Quantity: 1, Price: 20000, Currency: "CNY", SignedDate: now.UnixMilli(), PurchaseDate: now.UnixMilli(), Environment: "Sandbox"}
	if recordStore(ctx, db, receipt, owner, false) == nil {
		t.Fatal("sandbox credited production")
	}
	receipt.Environment = "Production"
	if recordStore(ctx, db, receipt, "00000000-0000-0000-0000-000000000000", false) == nil {
		t.Fatal("credited another reader")
	}
	apply := func() {
		t.Helper()
		if e := recordStore(ctx, db, receipt, owner, false); e != nil {
			t.Fatal(e)
		}
	}
	verify := func(plan string, n int) {
		t.Helper()
		var got string
		var credits int
		if e := db.QueryRow(ctx, "SELECT plan,credits FROM reader_memberships WHERE reader_id=$1", owner).Scan(&got, &credits); e != nil || got != plan || credits != n {
			t.Fatal(got, credits, e)
		}
	}
	apply()
	apply()
	verify("plus", 125)
	var count int
	var hours float64
	if e := db.QueryRow(ctx, "SELECT count(*),extract(epoch FROM max(ends_at-starts_at))/3600 FROM reader_store_purchases").Scan(&count, &hours); e != nil || count != 1 || hours != 720 {
		t.Fatal(count, hours, e)
	}
	// A later ordinary restore does not override or suppress a refund.
	receipt.SignedDate = now.Add(10 * time.Minute).UnixMilli()
	apply()
	receipt.RevocationDate = now.Add(time.Minute).UnixMilli()
	receipt.SignedDate = receipt.RevocationDate
	apply()
	verify("free", 10)
	receipt.RevocationDate = 0
	receipt.SignedDate = now.UnixMilli()
	apply()
	verify("free", 10)
	receipt.SignedDate = now.Add(2 * time.Minute).UnixMilli()
	if e := recordStore(ctx, db, receipt, owner, true); e != nil {
		t.Fatal(e)
	}
	verify("plus", 125)
	for _, id := range []string{"123451", "123452"} {
		receipt.TransactionID = id
		apply()
	}
	var overlapping bool
	if e := db.QueryRow(ctx, `SELECT EXISTS(SELECT FROM reader_paid_periods a JOIN reader_paid_periods b
 ON a.reader_id=b.reader_id AND a.starts_at<b.ends_at AND b.starts_at<a.ends_at AND a.starts_at<>b.starts_at)
 OR count(DISTINCT starts_at)<>3 OR max(ends_at)-min(starts_at)<>interval '2160 hours' FROM reader_paid_periods`).Scan(&overlapping); e != nil || overlapping {
		t.Fatal("overlapping membership periods", e)
	}
}

func TestStoreWalletLedger(t *testing.T) {
	db := storeTestDatabase(t)
	ctx := context.Background()
	t.Setenv("STOREKIT_ENVIRONMENT", "Sandbox")
	var wallet string
	err := db.QueryRow(ctx, `INSERT INTO billing_accounts(customer_ref,token_hash,currency,created_by)
       VALUES('sample-wallet',decode(repeat('01',32),'hex'),'CNY','test') RETURNING id`).Scan(&wallet)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().Add(-time.Minute).UnixMilli()
	tx := storeTransaction{TransactionID: "1230000", AppAccountToken: wallet, BundleID: storeBundle, Environment: "Sandbox", Type: "Consumable",
		ProductID: walletProduct, Quantity: 1, PurchaseDate: now, SignedDate: now, Price: 20000, Currency: "CNY"}
	balance := func(want int64, entries int) {
		t.Helper()
		var got int64
		var n int
		if err := db.QueryRow(ctx, "SELECT available_micros,(SELECT count(*) FROM billing_entries) FROM billing_balances WHERE id=$1", wallet).Scan(&got, &n); err != nil || got != want || n != entries {
			t.Fatal(got, n, err)
		}
	}
	apply := func(reverse bool) {
		t.Helper()
		if err := recordWallet(ctx, db, tx, reverse); err != nil {
			t.Fatal(err)
		}
	}
	apply(false)
	apply(false)
	balance(20_000_000, 1)
	tx.RevocationDate = now + 1
	tx.SignedDate = now + 1
	apply(false)
	apply(false)
	balance(0, 2)
	tx.RevocationDate = 0
	tx.SignedDate = now + 4
	apply(false)
	balance(0, 2)
	tx.SignedDate = now + 2
	apply(true)
	apply(true)
	balance(20_000_000, 3)
	tx.RevocationDate = now + 1
	tx.SignedDate = now + 1
	apply(false)
	balance(20_000_000, 3)
	tx.Environment = "Production"
	if err := recordWallet(ctx, db, tx, false); err == nil {
		t.Fatal("production payment credited in sandbox")
	}
	tx.Environment = "Sandbox"
	tx.ProductID = storeBundle + ".plus.30days"
	if err := recordWallet(ctx, db, tx, false); err == nil {
		t.Fatal("membership credited wallet")
	}
}
