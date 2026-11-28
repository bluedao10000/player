package knowledge_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	routing "anime-agent/api/internal/router"
	"anime-agent/api/internal/service/knowledge"
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

func TestArchiveLocations(t *testing.T) {
	const digest = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	check := func(want string) {
		t.Helper()
		if path, err := knowledge.ArchivePath("sha256:" + digest); err != nil || path != filepath.Join(want, digest) {
			t.Fatalf("snapshot path = %q, err = %v", path, err)
		}
		if path := modelusage.RecoveryDir(); path != filepath.Join(want, "model-usage-outbox") {
			t.Fatalf("outbox path = %q, want it under %q", path, want)
		}
	}
	home := t.TempDir()
	for _, env := range []string{"HOME", "home", "AppData", "XDG_CONFIG_HOME"} {
		t.Setenv(env, home)
	}
	t.Setenv("KNOWLEDGE_ARCHIVE_DIR", "")
	base, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	check(filepath.Join(base, "tidepost", "knowledge-archive"))
	for _, env := range []string{"HOME", "home", "AppData", "XDG_CONFIG_HOME"} {
		t.Setenv(env, "")
	}
	if path, err := knowledge.ArchivePath("sha256:" + digest); err == nil || path != "" {
		t.Fatalf("snapshot without a root = %q, err = %v", path, err)
	}
	if got := modelusage.RecoveryDir(); got != filepath.Join("tidepost", "knowledge-archive", "model-usage-outbox") {
		t.Fatalf("legacy outbox = %q", got)
	}
	for _, override := range []string{home, "relative archive"} {
		t.Setenv("KNOWLEDGE_ARCHIVE_DIR", override)
		check(override)
	}
}

// The offline image CLI can finish a paid request while PostgreSQL is unavailable.
// Recovery may see the terminal file before the start file.
func TestOfflineModelUsageImport(t *testing.T) {
	ctx := context.Background()
	db := testsupport.Database(t)
	// Seed the historical path independently of the resolver used for replay.
	writer := t.TempDir()
	t.Chdir(writer)
	for _, env := range []string{"HOME", "home", "AppData", "XDG_CONFIG_HOME", "KNOWLEDGE_ARCHIVE_DIR", "KNOWLEDGE_LEGACY_USAGE_OUTBOX"} {
		t.Setenv(env, "")
	}
	dir := filepath.Join("tidepost", "knowledge-archive", "model-usage-outbox")
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
	legacy := filepath.Join(writer, dir)
	t.Chdir(t.TempDir())
	for _, wrong := range []string{"", dir, filepath.Join(legacy, "missing"), filepath.Join(legacy, e.ID+"-ok.json")} {
		t.Setenv("KNOWLEDGE_LEGACY_USAGE_OUTBOX", wrong)
		if err := modelusage.ReplayConfigured(ctx, db); err == nil || !strings.Contains(err.Error(), "KNOWLEDGE_LEGACY_USAGE_OUTBOX") {
			t.Fatalf("legacy path %q: got %v", wrong, err)
		}
	}
	if err := modelusage.Replay(ctx, db, dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("manual import from wrong cwd: %v", err)
	}
	files, err := os.ReadDir(legacy)
	if err != nil || len(files) != 2 {
		t.Fatalf("wrong path acknowledged historical files: %v, %v", files, err)
	}
	var count, total int
	if err := db.QueryRow(ctx, sqltest.ReceiptTotals).Scan(&count, &total); err != nil || count != 0 {
		t.Fatal(count, err)
	}

	t.Setenv("KNOWLEDGE_LEGACY_USAGE_OUTBOX", legacy)
	if err := modelusage.ReplayConfigured(ctx, db); err != nil {
		t.Fatal(err)
	}
	// Re-delivery after a lost acknowledgement must agree with the committed event.
	if err := os.WriteFile(filepath.Join(legacy, e.ID+"-ok.json"), receipt, 0600); err != nil {
		t.Fatal(err)
	}
	if err := modelusage.ReplayConfigured(ctx, db); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(ctx, sqltest.OfflineUsageTotals).Scan(&count, &total); err != nil || count != 1 || total != 17 {
		t.Fatal(count, total, err)
	}
	files, err = os.ReadDir(legacy)
	if err != nil || len(files) != 0 {
		t.Fatal("committed files not acknowledged", err)
	}
}

func TestUsageReplayIndependentDirectories(t *testing.T) {
	ctx := context.Background()
	db := testsupport.Database(t)
	t.Setenv("KNOWLEDGE_ARCHIVE_DIR", t.TempDir())
	t.Setenv("KNOWLEDGE_LEGACY_USAGE_OUTBOX", "")
	if err := modelusage.ReplayConfigured(ctx, db); err != nil {
		t.Fatalf("unused active outbox: %v", err)
	}
	active, legacy := modelusage.RecoveryDir(), t.TempDir()
	t.Setenv("KNOWLEDGE_LEGACY_USAGE_OUTBOX", legacy)
	if err := os.Mkdir(active, 0700); err != nil {
		t.Fatal(err)
	}
	for _, dirs := range []struct{ failing, healthy string }{{active, legacy}, {legacy, active}} {
		start := time.Now().UTC()
		e := modelusage.Event{ID: modelusage.NewID(), Purpose: "image", Operation: "images/edits", Subject: "preview:fixture", Endpoint: "https://fixture.invalid/v1", Model: "art", Started: start, Finished: &start, Outcome: "ok", Usage: json.RawMessage(`{"total_tokens":7}`)}
		if err := modelusage.Deliver(ctx, nil, dirs.healthy, e); err != nil {
			t.Fatal(err)
		}
		// This decodes, but cannot be saved without a model call identity.
		failed := filepath.Join(dirs.failing, "bad.json")
		if err := os.WriteFile(failed, []byte(`{}`), 0600); err != nil {
			t.Fatal(err)
		}
		if err := modelusage.ReplayConfigured(ctx, db); err == nil || !strings.Contains(err.Error(), failed) {
			t.Fatalf("missing failing receipt path: %v", err)
		}
		if data, err := os.ReadFile(failed); err != nil || string(data) != `{}` {
			t.Fatalf("failed receipt was not retained: %v", err)
		}
		if files, err := os.ReadDir(dirs.healthy); err != nil || len(files) != 0 {
			t.Fatalf("healthy outbox: %v, %v", files, err)
		}
		if err := os.Remove(failed); err != nil {
			t.Fatal(err)
		}
	}
	var count, total int
	if err := db.QueryRow(ctx, sqltest.OfflineUsageTotals).Scan(&count, &total); err != nil || count != 2 || total != 14 {
		t.Fatal(count, total, err)
	}
}
