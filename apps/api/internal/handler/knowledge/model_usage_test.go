package knowledge_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	routing "anime-agent/api/internal/router"
	"anime-agent/api/internal/service/modelusage"
	"anime-agent/api/internal/testsupport"
	sqltest "anime-agent/api/internal/testsupport/fixtures"
)

// Transport replay and business rollback must not double count or lose consumption.
func TestProviderUsageReconciliation(t *testing.T) {
	db := testsupport.Database(t)
	ctx := context.Background()
	start := time.Now().UTC().Truncate(time.Second)
	initial := modelusage.Event{ID: modelusage.NewID(), Purpose: "vision", Operation: "review", Subject: "media:17", Endpoint: "https://fixture.invalid/v1", Account: "org1", Model: "requested", Outcome: "started", Started: start}
	done := initial
	done.Outcome = "ok"
	done.Finished = &start
	done.Request = "req"
	done.Resolved = "snapshot"
	done.Usage = json.RawMessage(`{"input_tokens":8,"input_tokens_details":{"cached_tokens":3},"output_tokens":2,"model_text":"must never persist"}`)
	later := done
	later.ID = modelusage.NewID()
	later.Response = "resp"
	later.Started = start.Add(24 * time.Hour)
	later.Finished = &later.Started
	sequence := []modelusage.Delivery{{ID: "s", Event: initial}, {ID: "f", Event: done}, {ID: "another-attempt", Event: later}}
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			json.NewEncoder(w).Encode(sequence)
			return
		}
		attempts++
		if attempts == 1 {
			w.WriteHeader(503)
			return
		}
		var ack []string
		if json.NewDecoder(r.Body).Decode(&ack) != nil || len(ack) != 3 {
			t.Error("bad ack")
		}
		w.WriteHeader(204)
	}))
	defer server.Close()
	if e := modelusage.Pull(ctx, db, server.URL); e == nil {
		t.Fatal("expected ack failure")
	}
	if e := modelusage.Pull(ctx, db, server.URL); e != nil {
		t.Fatal(e)
	}
	save := func(e modelusage.Event) {
		t.Helper()
		if err := modelusage.Save(ctx, db, e); err != nil {
			t.Fatal(err)
		}
	}
	save(initial) // Late start cannot undo a receipt.
	check := func(wantN, wantUsage int) {
		t.Helper()
		var n, total int
		err := db.QueryRow(ctx, sqltest.ReceiptTotals).Scan(&n, &total)
		if err != nil || n != wantN || total != wantUsage {
			t.Fatal(n, total, err)
		}
	}
	check(2, 10)
	badReplay := done
	badReplay.Subject = "media:unexpected"
	if err := modelusage.Save(ctx, db, badReplay); err == nil {
		t.Fatal("accepted conflicting event identity")
	}
	other := later
	other.ID = modelusage.NewID()
	other.Account = "org2"
	save(other)
	check(3, 20)
	var safe bool
	if err := db.QueryRow(ctx, sqltest.SanitizedUsage, initial.ID).Scan(&safe); err != nil || !safe {
		t.Fatal(safe, err)
	}
	missing := initial
	missing.ID = modelusage.NewID()
	missing.Purpose = "image"
	missing.Outcome = "ok"
	missing.Finished = &start
	n := 1
	missing.Images = &n
	save(missing)
	conflicting := later
	conflicting.ID = modelusage.NewID()
	conflicting.Usage = json.RawMessage(`{"prompt_tokens":99,"completion_tokens":3}`)
	save(conflicting)
	check(5, 10)
	var invalid, unreported int
	if err := db.QueryRow(ctx, sqltest.ConflictingReceipts).Scan(&invalid, &unreported); err != nil || invalid != 3 || unreported != 1 {
		t.Fatal(invalid, unreported, err)
	}
	zero := missing
	zero.ID = modelusage.NewID()
	zero.Usage = json.RawMessage(`{"total_tokens":0}`)
	save(zero)
	partial := missing
	partial.ID = modelusage.NewID()
	partial.Usage = json.RawMessage(`{"prompt_tokens":3}`)
	save(partial)
	var explicitZero, partialUsage int
	err := db.QueryRow(ctx, sqltest.PartialReceipts).Scan(&explicitZero, &partialUsage)
	if err != nil || explicitZero != 1 || partialUsage != 1 {
		t.Fatal(explicitZero, partialUsage, err)
	}
	root := gin.New()
	routing.Register(root, db)
	token := "12345678901234567890123456789012"
	t.Setenv("KNOWLEDGE_ADMIN_TOKEN", token)
	t.Setenv("KNOWLEDGE_ADMIN_OPERATOR", "fixture")
	path := "/api/v1/admin/knowledge/model-usage"
	for _, q := range []struct {
		path   string
		auth   bool
		status int
	}{{path, false, 401}, {path, true, 200}, {path + "/calls?subject=media:17", true, 200}, {path + "?from=invalid", true, 400}} {
		req := httptest.NewRequest("GET", q.path, nil)
		if q.auth {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		root.ServeHTTP(w, req)
		if w.Code != q.status {
			t.Fatal(w.Code, w.Body.String())
		}
	}
}

// The offline image CLI can finish a paid request while PostgreSQL is unavailable.
// Recovery may see the terminal file before the start file.
func TestOfflineModelUsageImport(t *testing.T) {
	ctx := context.Background()
	db := testsupport.Database(t)
	dir := t.TempDir()
	start := time.Now().UTC()
	e := modelusage.Event{ID: modelusage.NewID(), Purpose: "image", Operation: "images/edits", Subject: "preview:fixture", Endpoint: "https://fixture.invalid/v1", Model: "art", Started: start, Outcome: "started"}
	if err := modelusage.Deliver(ctx, nil, dir, e); err != nil {
		t.Fatal(err)
	}
	e.Outcome = "ok"
	e.Finished = &start
	e.Usage = json.RawMessage(`{"total_tokens":17,"payload":"private-content"}`)
	if err := modelusage.Deliver(ctx, nil, dir, e); err != nil {
		t.Fatal(err)
	}
	receipt, err := os.ReadFile(filepath.Join(dir, e.ID+"-ok.json"))
	if err != nil || strings.Contains(string(receipt), "private-content") {
		t.Fatal("invalid offline receipt", err)
	}
	for i := 0; i < 2; i++ {
		if err := modelusage.Replay(ctx, db, dir); err != nil {
			t.Fatal(err)
		}
	}
	var count, total int
	if err := db.QueryRow(ctx, sqltest.OfflineUsageTotals).Scan(&count, &total); err != nil || count != 1 || total != 17 {
		t.Fatal(count, total, err)
	}
	files, err := os.ReadDir(dir)
	if err != nil || len(files) != 0 {
		t.Fatal("committed files not acknowledged", err)
	}
}
