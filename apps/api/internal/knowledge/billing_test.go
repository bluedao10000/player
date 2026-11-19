package knowledge

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"anime-agent/api/internal/billing"
	"anime-agent/api/internal/modelusage"
	"github.com/gin-gonic/gin"
)

func TestCustomerBillingAndReconciliation(t *testing.T) {
	db := testStore(t)
	ctx := context.Background()
	t.Setenv("KNOWLEDGE_ARCHIVE_DIR", t.TempDir())
	admin := strings.Repeat("A", 36)
	customer := strings.Repeat("B", 36)
	other := strings.Repeat("C", 36)
	t.Setenv("KNOWLEDGE_ADMIN_TOKEN", admin)
	t.Setenv("KNOWLEDGE_ADMIN_OPERATOR", "test-operator")
	r := gin.New()
	Routes(r, db)
	call := func(method, path, token string, body any, want int) map[string]any {
		t.Helper()
		raw, _ := json.Marshal(body)
		req := httptest.NewRequest(method, path, bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != want {
			t.Fatalf("%s %s -> %d expected %d, %s", method, path, w.Code, want, w.Body.String())
		}
		out := map[string]any{}
		if w.Code == 200 && !strings.HasSuffix(path, "/image") {
			if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
				t.Fatal(err)
			}
		}
		return out
	}
	const a = "/api/v1/admin/knowledge/billing"
	const u = "/api/v1/billing"
	for _, key := range []string{customer, other} {
		got := call("POST", a+"/accounts", admin, gin.H{"customer_ref": key[:1], "currency": "CNY", "token_sha256": fmt.Sprintf("%x", sha256.Sum256([]byte(key)))}, 200)
		if got["id"] == nil {
			t.Fatal(got)
		}
	}
	id := call("GET", u+"/account", customer, nil, 200)["id"].(string)
	call("GET", u+"/account", admin, nil, 401)
	call("GET", a+"/reconciliation", customer, nil, 401)
	credit := a + "/accounts/" + id + "/credits"
	for i := 0; i < 2; i++ {
		call("POST", credit, admin, gin.H{"reference": "deposit-1", "currency": "CNY", "amount_micros": 1000}, 200)
	}
	call("POST", credit, admin, gin.H{"reference": "deposit-1", "currency": "CNY", "amount_micros": 2000}, 409)
	stamp := time.Now().UTC()
	base := "https://fixture.invalid/v1"
	rate := gin.H{"kind": "retail", "endpoint": base, "provider_account": "studio", "model": "art", "currency": "CNY", "effective_at": stamp.Add(-time.Hour), "unit": "split_tokens", "input_micros": 2000000, "output_micros": 3000000, "cached_micros": 1000000, "cap_micros": 800}
	retail := call("POST", a+"/rates", admin, rate, 200)["id"].(string)
	rate["kind"] = "cost"
	rate["model"] = "actual"
	rate["cap_micros"] = 0
	rate["input_micros"] = 1000000
	rate["output_micros"] = 1000000
	rate["cached_micros"] = 1000000
	call("POST", a+"/rates", admin, rate, 200)
	if call("GET", u+"/quote", customer, nil, 200)["rate"].(map[string]any)["id"] != retail {
		t.Fatal("quote")
	}
	spec := gin.H{"size": "1024x1024", "brief": gin.H{"original_subject": "fictional character", "visual_bible": "blue", "scene": "a pier", "composition": "portrait", "art_direction": "watercolour"}}
	req := gin.H{"client_key": "preview-1", "rate_id": retail, "request": spec}
	order := call("POST", u+"/orders", customer, req, 200)["id"].(string)
	if call("POST", u+"/orders", customer, req, 200)["id"] != order {
		t.Fatal("double reservation")
	}
	call("POST", u+"/orders", customer, gin.H{"client_key": "unfunded", "rate_id": retail, "request": spec}, 402)
	call("GET", u+"/orders/"+order, other, nil, 404)
	if e := billing.Claim(ctx, db, order, base, "studio", "art"); e != nil {
		t.Fatal(e)
	}
	call("POST", u+"/orders/"+order+"/cancel", customer, nil, 409)
	// Two observations of the same provider exchange, including reply redelivery.
	n := 1
	now := time.Now().UTC()
	e := modelusage.Event{ID: modelusage.NewID(), Purpose: "image", Endpoint: base, Account: "studio", Model: "art", Resolved: "actual", Operation: "images/generations", Size: "1024x1024", Subject: "order:" + order, Outcome: "started", Started: now}
	save := func(e modelusage.Event) {
		t.Helper()
		if err := modelusage.Save(ctx, db, e); err != nil {
			t.Fatal(err)
		}
	}
	save(e)
	tmp := filepath.Join(t.TempDir(), "result.png")
	if err := os.WriteFile(tmp, []byte("fixture-image"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := billing.StorePreview(order, tmp); err != nil {
		t.Fatal(err)
	}
	call("POST", a+"/orders/"+order+"/settle", admin, nil, 409)
	e.Finished = &now
	e.Outcome = "ok"
	e.Response = "resp"
	e.Images = &n
	e.Usage = json.RawMessage(`{"input_tokens":11,"output_tokens":2,"input_tokens_details":{"cached_tokens":3}}`)
	save(e)
	save(e)
	e.ID = modelusage.NewID()
	save(e)
	// Retail version changes after admission; settlement must retain the original quote.
	rate["kind"] = "retail"
	rate["model"] = "art"
	rate["cap_micros"] = 700
	rate["effective_at"] = now
	call("POST", a+"/rates", admin, rate, 200)
	call("POST", u+"/orders", customer, gin.H{"client_key": "stale", "rate_id": retail, "request": spec}, 409)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := billing.FinalizePreview(ctx, db, order, "fixture"); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	got := call("GET", u+"/orders/"+order, customer, nil, 200)
	if got["charge_micros"] != float64(25) || got["status"] != "settled" || got["rate_id"] != retail {
		t.Fatal(got)
	}
	got = call("GET", u+"/account", customer, nil, 200)
	if got["held_micros"] != float64(0) || got["available_micros"] != float64(975) {
		t.Fatal(got)
	}
	call("GET", u+"/orders/"+order+"/image", other, nil, 404)
	call("GET", u+"/orders/"+order+"/image", customer, nil, 200)
	day := now.Format(time.DateOnly)
	report := a + "/reconciliation?day=" + day + "&currency=CNY"
	group := func() map[string]any {
		return call("GET", report, admin, nil, 200)["groups"].([]any)[0].(map[string]any)
	}
	if g := group(); g["status"] != "local_only" || g["known_cost_micros"] != float64(13) {
		t.Fatal(g)
	}
	statement := gin.H{"day": day, "endpoint": base, "provider_account": "studio", "model": "actual", "currency": "CNY", "reference": "invoice-1", "amount_micros": 13}
	for i := 0; i < 2; i++ {
		call("POST", a+"/statements", admin, statement, 204)
	}
	if g := group(); g["status"] != "matched" || g["receipts"] != float64(1) || g["margin_micros"] != float64(12) {
		t.Fatal(g)
	}
	statement["amount_micros"] = 12
	call("POST", a+"/statements", admin, statement, 409)
	statement["reference"] = "invoice-correction"
	statement["replaces"] = "invoice-1"
	call("POST", a+"/statements", admin, statement, 204)
	if g := group(); g["status"] != "difference" || g["difference_micros"] != float64(-1) || g["margin_micros"] != nil {
		t.Fatal(g)
	}
	statement["replaces"] = "invoice-correction"
	statement["reference"] = "invoice-final"
	statement["amount_micros"] = 13
	call("POST", a+"/statements", admin, statement, 204)
	for i := 0; i < 2; i++ {
		call("POST", a+"/orders/"+order+"/refund", admin, gin.H{"reason": "customer refund"}, 200)
	}
	if g := group(); g["net_revenue_micros"] != float64(0) || g["margin_micros"] != float64(-13) {
		t.Fatal(g)
	}
	if got = call("GET", u+"/account", customer, nil, 200); got["balance_micros"] != float64(1000) {
		t.Fatal(got)
	}
	// Platform receipts remain costs only, including a missing usage and a real zero.
	e.ID = modelusage.NewID()
	e.Subject = "preview:platform"
	e.Response = "unknown-usage"
	e.Usage = nil
	save(e)
	if g := group(); g["status"] != "pending" || g["margin_micros"] != nil || g["pending"] != float64(1) {
		t.Fatal(g)
	}
	statement["model"] = "statement-only"
	statement["reference"] = "other-line"
	statement["replaces"] = ""
	call("POST", a+"/statements", admin, statement, 204)
	groups := call("GET", report, admin, nil, 200)["groups"].([]any)
	if g := groups[1].(map[string]any); g["status"] != "supplier_only" || g["margin_micros"] != nil {
		t.Fatal(g)
	}
	call("GET", a+"/reconciliation?day=broken&currency=CNY", admin, nil, 400)
}

func TestPaidPreviewCommand(t *testing.T) {
	db := testStore(t)
	ctx := context.Background()
	t.Setenv("KNOWLEDGE_ARCHIVE_DIR", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewGray(image.Rect(0, 0, 1024, 1024))); err != nil {
		t.Fatal(err)
	}
	var dispatch atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dispatch.Add(1)
		var body struct{ Prompt, Size string }
		if r.URL.Path != "/images/generations" || json.NewDecoder(r.Body).Decode(&body) != nil || body.Size != "1024x1024" || !strings.Contains(body.Prompt, "order-fixture") {
			t.Error("provider request did not match authorized brief")
		}
		json.NewEncoder(w).Encode(gin.H{"id": "r-1", "data": []gin.H{{"b64_json": base64.StdEncoding.EncodeToString(buf.Bytes())}}, "usage": gin.H{"total_tokens": 350}})
	}))
	defer srv.Close()
	t.Setenv("MEDIA_AI_BASE_URL", srv.URL)
	t.Setenv("MEDIA_AI_MODEL", "fixture")
	t.Setenv("MEDIA_AI_ACCOUNT", "studio")
	t.Setenv("MEDIA_AI_API_KEY", "fixture")
	var account, rate string
	if e := db.QueryRow(ctx, `INSERT INTO billing_accounts(customer_ref,token_hash,currency,created_by) VALUES ('cli',decode(repeat('00',32),'hex'),'USD','test') RETURNING id::text`).Scan(&account); e != nil {
		t.Fatal(e)
	}
	if e := db.QueryRow(ctx, `INSERT INTO billing_rates(kind,endpoint,provider_account,model,currency,effective_at,unit,unit_micros,cap_micros,created_by)
 VALUES('retail',$1,'studio','fixture','USD',now(),'total_tokens',1000000,100,'test') RETURNING id::text`, srv.URL).Scan(&rate); e != nil {
		t.Fatal(e)
	}
	if e := billing.Credit(ctx, db, account, "cli-deposit", "test", "USD", 100); e != nil {
		t.Fatal(e)
	}
	spec := billing.PreviewRequest{Size: "1024x1024", Brief: billing.ArtBrief{Subject: "order-fixture", VisualBible: "v", Scene: "s", Composition: "c", ArtDirection: "a"}}
	type result struct {
		id  string
		err error
	}
	claims := make(chan result, 2)
	for _, key := range []string{"one", "two"} {
		go func() { id, e := billing.Reserve(ctx, db, account, key, rate, spec); claims <- result{id, e} }()
	}
	var order string
	success, denied := 0, 0
	for range 2 {
		r := <-claims
		if r.err == nil {
			order = r.id
			success++
		} else if errors.Is(r.err, billing.ErrFunds) {
			denied++
		} else {
			t.Fatal(r.err)
		}
	}
	if success != 1 || denied != 1 {
		t.Fatal("funds double reserved")
	}
	for range 2 {
		cmd := exec.CommandContext(ctx, "go", "run", "../../cmd/media", "generate-preview", "--order", order)
		if out, e := cmd.CombinedOutput(); e != nil {
			t.Fatalf("private preview: %v %s", e, out)
		}
	}
	var charge, uncapped int
	var owner string
	if e := db.QueryRow(ctx, `SELECT o.charge_micros,(o.units->>'uncapped_micros')::int,m.cost_owner FROM billing_orders o JOIN model_calls m ON m.id=o.receipt_id WHERE o.id=$1`, order).Scan(&charge, &uncapped, &owner); e != nil || charge != 100 || uncapped != 350 || owner != "customer" || dispatch.Load() != 1 {
		t.Fatal(charge, uncapped, owner, dispatch.Load(), e)
	}
	f, e := os.Open(billing.PreviewPath(order))
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	img, e := png.DecodeConfig(f)
	if e != nil || img.Width != 1024 {
		t.Fatal(img, e)
	}
}
