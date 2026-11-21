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
	"math/big"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"anime-agent/api/internal/testsupport"
	sqltest "anime-agent/database/queries/fixtures"
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
		var tx StoreTransaction
		err := VerifyStoreJWS(signed, &tx, &tx.SignedDate)
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

func TestStoreReceiptPeriods(t *testing.T) {
	db := testsupport.Database(t)
	ctx := context.Background()
	var owner string
	e := db.QueryRow(ctx, sqltest.CreateStoreReader).Scan(&owner)
	if e != nil {
		t.Fatal(e)
	}
	t.Setenv("STOREKIT_ENVIRONMENT", "Production")
	now := time.Now().Add(-time.Hour)
	receipt := StoreTransaction{TransactionID: "123450", BundleID: storeBundle, ProductID: storeBundle + ".plus.30days", Type: "Non-Renewing Subscription", AppAccountToken: owner, Quantity: 1, Price: 20000, Currency: "CNY", SignedDate: now.UnixMilli(), PurchaseDate: now.UnixMilli(), Environment: "Sandbox"}
	if RecordStore(ctx, db, receipt, owner, false) == nil {
		t.Fatal("sandbox credited production")
	}
	receipt.Environment = "Production"
	if RecordStore(ctx, db, receipt, "00000000-0000-0000-0000-000000000000", false) == nil {
		t.Fatal("credited another reader")
	}
	apply := func() {
		t.Helper()
		if e := RecordStore(ctx, db, receipt, owner, false); e != nil {
			t.Fatal(e)
		}
	}
	verify := func(plan string, n int) {
		t.Helper()
		var got string
		var credits int
		if e := db.QueryRow(ctx, sqltest.ReadPlanCredits, owner).Scan(&got, &credits); e != nil || got != plan || credits != n {
			t.Fatal(got, credits, e)
		}
	}
	apply()
	apply()
	verify("plus", 125)
	var count int
	var hours float64
	if e := db.QueryRow(ctx, sqltest.PurchasePeriods).Scan(&count, &hours); e != nil || count != 1 || hours != 720 {
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
	if e := RecordStore(ctx, db, receipt, owner, true); e != nil {
		t.Fatal(e)
	}
	verify("plus", 125)
	for _, id := range []string{"123451", "123452"} {
		receipt.TransactionID = id
		apply()
	}
	var overlapping bool
	if e := db.QueryRow(ctx, sqltest.HasOverlappingPeriods).Scan(&overlapping); e != nil || overlapping {
		t.Fatal("overlapping membership periods", e)
	}
}
