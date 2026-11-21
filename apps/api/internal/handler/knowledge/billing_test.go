package knowledge_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	routing "anime-agent/api/internal/router"
	"anime-agent/api/internal/service/modelusage"
	"anime-agent/api/internal/testsupport"
)

func TestSupplierCostReconciliation(t *testing.T) {
	db := testsupport.Database(t)
	ctx := context.Background()
	t.Setenv("KNOWLEDGE_ARCHIVE_DIR", t.TempDir())
	admin := strings.Repeat("A", 36)
	unauthorized := strings.Repeat("B", 36)

	t.Setenv("KNOWLEDGE_ADMIN_TOKEN", admin)
	t.Setenv("KNOWLEDGE_ADMIN_OPERATOR", "test-operator")
	r := gin.New()
	routing.Register(r, db)
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
		if w.Code == 200 {
			if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
				t.Fatal(err)
			}
		}
		return out
	}
	const a = "/api/v1/admin/knowledge/billing"
	call("GET", a+"/reconciliation", unauthorized, nil, 401)
	stamp := time.Now().UTC()
	base := "https://fixture.invalid/v1"
	rate := gin.H{"kind": "cost", "endpoint": base, "provider_account": "studio", "model": "actual", "currency": "CNY", "effective_at": stamp.Add(-time.Hour), "unit": "split_tokens", "input_micros": 1000000, "output_micros": 1000000, "cached_micros": 1000000}
	call("POST", a+"/rates", admin, rate, 200)
	n := 1
	now := time.Now().UTC()
	e := modelusage.Event{ID: modelusage.NewID(), Purpose: "image", Endpoint: base, Account: "studio", Model: "art", Resolved: "actual", Operation: "images/generations", Size: "1024x1024", Subject: "episode:fixture", Outcome: "started", Started: now}
	save := func(e modelusage.Event) {
		t.Helper()
		if err := modelusage.Save(ctx, db, e); err != nil {
			t.Fatal(err)
		}
	}
	save(e)
	e.Finished = &now
	e.Outcome = "ok"
	e.Response = "resp"
	e.Images = &n
	e.Usage = json.RawMessage(`{"input_tokens":11,"output_tokens":2,"input_tokens_details":{"cached_tokens":3}}`)
	save(e)
	save(e)
	e.ID = modelusage.NewID()
	save(e)
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
	if g := group(); g["status"] != "matched" || g["receipts"] != float64(1) {
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
	// Platform receipts remain costs only, including a missing usage and a real zero.
	e.ID = modelusage.NewID()
	e.Subject = "episode:platform"
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
