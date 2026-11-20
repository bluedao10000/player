package knowledge

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func signText(v url.Values, skipType bool) []byte {
	keys := make([]string, 0, len(v))
	for k := range v {
		if k != "sign" && (!skipType || k != "sign_type") {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+v.Get(k))
	}
	h := sha256.Sum256([]byte(strings.Join(parts, "&")))
	return h[:]
}

func TestAlipayPageCheckout(t *testing.T) {
	db := testStore(t)
	ctx := context.Background()
	merchant, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	provider, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	public, err := x509.MarshalPKIXPublicKey(&provider.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("ALIPAY_APP_ID", "test-app")
	t.Setenv("ALIPAY_SELLER_ID", "test-seller")
	t.Setenv("ALIPAY_NOTIFY_URL", "https://reader.example/api/billing/alipay/notify")
	t.Setenv("ALIPAY_PRIVATE_KEY", base64.StdEncoding.EncodeToString(x509.MarshalPKCS1PrivateKey(merchant)))
	t.Setenv("ALIPAY_PUBLIC_KEY", base64.StdEncoding.EncodeToString(public))
	const token = "fixture-billing-customer-key-00000001"
	digest := sha256.Sum256([]byte(token))
	var account string
	if err = db.QueryRow(ctx, "INSERT INTO billing_accounts(customer_ref,currency,token_hash,created_by) VALUES('reader','CNY',$1,'test') RETURNING id::text", digest[:]).Scan(&account); err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	Routes(r, db)
	checkout := func(key string, fen int, want int) string {
		t.Helper()
		req := httptest.NewRequest("POST", "/api/v1/billing/alipay/topups", strings.NewReader(fmt.Sprintf(`{"client_key":%q,"amount_fen":%d}`, key, fen)))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != want {
			t.Fatalf("checkout %d %s", w.Code, w.Body.String())
		}
		if want != 200 {
			return ""
		}
		var out struct {
			URL string `json:"checkout_url"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		u, err := url.Parse(out.URL)
		if err != nil {
			t.Fatal(err)
		}
		q := u.Query()
		sig, err := base64.StdEncoding.DecodeString(q.Get("sign"))
		if err != nil {
			t.Fatal(err)
		}
		if err = rsa.VerifyPKCS1v15(&merchant.PublicKey, crypto.SHA256, signText(q, false), sig); err != nil {
			t.Fatal("invalid merchant signature")
		}
		var b map[string]string
		if err = json.Unmarshal([]byte(q.Get("biz_content")), &b); err != nil {
			t.Fatal(err)
		}
		if u.Host != "openapi.alipay.com" || q.Get("method") != "alipay.trade.page.pay" || q.Get("return_url") != "https://reader.example/commissions" || b["product_code"] != "FAST_INSTANT_TRADE_PAY" || b["total_amount"] != "12.34" {
			t.Fatal("wrong Page Pay contract")
		}
		return b["out_trade_no"]
	}
	const k = "76110917-1704-4f80-a523-fe9b39cd5571"
	no := checkout(k, 1234, 200)
	if checkout(k, 1234, 200) != no {
		t.Fatal("duplicated order")
	}
	checkout(k, 1235, 409)
	v := url.Values{"app_id": {"test-app"}, "seller_id": {"test-seller"}, "out_trade_no": {no}, "trade_no": {"2026001234"}, "total_amount": {"12.34"}, "trade_status": {"TRADE_SUCCESS"}, "notify_id": {"fixture-id"}, "notify_type": {"trade_status_sync"}, "sign_type": {"RSA2"}}
	sign := func() {
		s, e := rsa.SignPKCS1v15(rand.Reader, provider, crypto.SHA256, signText(v, true))
		if e != nil {
			t.Fatal(e)
		}
		v.Set("sign", base64.StdEncoding.EncodeToString(s))
	}
	notify := func(raw string, want int) {
		req := httptest.NewRequest("POST", "/api/billing/alipay/notify", strings.NewReader(raw))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != want || (want == 200 && w.Body.String() != "success") {
			t.Errorf("notify %d %s", w.Code, w.Body.String())
		}
	}
	for field, bad := range map[string]string{"total_amount": "12.35", "seller_id": "other", "app_id": "other", "gmt_refund": "2026-01-01", "sign_type": "RSA"} {
		old := v.Get(field)
		v.Set(field, bad)
		sign()
		notify(v.Encode(), 400)
		if old == "" {
			v.Del(field)
		} else {
			v.Set(field, old)
		}
	}
	v.Set("sign", "bad")
	notify(v.Encode(), 400)
	sign()
	notify(v.Encode()+"&total_amount=12.34", 400)
	var count int
	if e := db.QueryRow(ctx, "SELECT count(*) FROM billing_entries").Scan(&count); e != nil || count != 0 {
		t.Fatal("invalid notification credited", count, e)
	}
	var wg sync.WaitGroup
	raw := v.Encode()
	for range 6 {
		wg.Add(1)
		go func() { defer wg.Done(); notify(raw, 200) }()
	}
	wg.Wait()
	v.Set("trade_status", "TRADE_FINISHED")
	sign()
	notify(v.Encode(), 200)
	v.Set("out_trade_no", checkout("450cc00b-e39a-43f0-b861-bd49205822cc", 1234, 200))
	sign()
	notify(v.Encode(), 400)
	var amount int64
	if e := db.QueryRow(ctx, "SELECT count(*),coalesce(sum(amount_micros),0) FROM billing_entries WHERE account_id=$1", account).Scan(&count, &amount); e != nil || count != 1 || amount != 12340000 {
		t.Fatal(count, amount, e)
	}
	checkout(k, 1234, 409)

	t.Run("reader membership receipt", func(t *testing.T) {
		var owner string
		if err := db.QueryRow(ctx, "INSERT INTO reader_identities(email) VALUES('membership@example.test') RETURNING id").Scan(&owner); err != nil {
			t.Fatal(err)
		}
		session := strings.Repeat("1", 64)
		hash := sha256.Sum256([]byte(session))
		if _, err := db.Exec(ctx, "INSERT INTO reader_sessions VALUES($1,$2,now()+interval '1 hour')", hash[:], owner); err != nil {
			t.Fatal(err)
		}
		api := func(method, path, body string, want int) map[string]any {
			t.Helper()
			req := httptest.NewRequest(method, "/api/v1/reader/"+path, strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+session)
			req.Header.Set("X-Reader-ID", owner)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != want {
				t.Fatalf("%s %d %s", path, w.Code, w.Body.String())
			}
			var v map[string]any
			if want == 200 {
				if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
					t.Fatal(err)
				}
			}
			return v
		}
		if free := api("GET", "membership", "", 200); free["credits"] != float64(10) || free["plan"] != "free" {
			t.Fatal("missing Free allowance")
		}
		api("POST", "membership/checkout", `{"plan":"plus","expectedFen":1}`, 409)
		buy := func() string {
			data := api("POST", "membership/checkout", `{"plan":"plus","expectedFen":2000}`, 200)
			u, err := url.Parse(data["checkout_url"].(string))
			if err != nil {
				t.Fatal(err)
			}
			var biz map[string]string
			err = json.Unmarshal([]byte(u.Query().Get("biz_content")), &biz)
			if err != nil || u.Query().Get("method") != "alipay.trade.page.pay" || biz["product_code"] != "FAST_INSTANT_TRADE_PAY" || biz["total_amount"] != "20.00" || u.Query().Get("return_url") != "https://reader.example/me?payment=return" {
				t.Fatal("invalid plan checkout")
			}
			return biz["out_trade_no"]
		}
		number := buy()
		if !strings.HasPrefix(number, "MP") || buy() != number {
			t.Fatal("duplicate checkout")
		}
		api("POST", "membership/checkout", `{"plan":"pro","expectedFen":9900}`, 409)
		v.Set("trade_status", "TRADE_SUCCESS")
		v.Set("out_trade_no", number)
		v.Set("trade_no", "2026001235")
		v.Set("gmt_payment", time.Now().In(time.FixedZone("CST", 8*3600)).Format(time.DateTime))
		v.Set("total_amount", "99.00")
		sign()
		notify(v.Encode(), 400)
		v.Set("total_amount", "20.00")
		sign()
		raw := v.Encode()
		for range 4 {
			wg.Add(1)
			go func() { defer wg.Done(); notify(raw, 200) }()
		}
		wg.Wait()
		data := api("GET", "membership", "", 200)
		if data["plan"] != "plus" || data["credits"] != float64(125) || len(data["orders"].([]any)) != 1 {
			t.Fatal("membership not activated exactly once", data)
		}
		api("POST", "membership/checkout", `{"plan":"plus","expectedFen":2000}`, 409)
		var n int
		if err := db.QueryRow(ctx, "SELECT count(*) FROM reader_plan_orders WHERE reader_id=$1 AND ends_at=starts_at+interval '720 hours'", owner).Scan(&n); err != nil || n != 1 {
			t.Fatal(n, err)
		}
		v.Set("trade_status", "TRADE_CLOSED")
		v.Set("refund_fee", "20.00")
		sign()
		notify(v.Encode(), 200)
		if got := api("GET", "membership", "", 200); got["plan"] != "free" {
			t.Fatal("refunded term still active")
		}
	})
}
