package scheduler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"anime-agent/api/internal/service/modelusage"
	"anime-agent/api/internal/testsupport"
	sqltest "anime-agent/api/internal/testsupport/fixtures"
)

func TestLaneUsageRecovery(t *testing.T) {
	db := testsupport.Database(t)
	ctx := t.Context()
	t.Setenv("KNOWLEDGE_ARCHIVE_DIR", t.TempDir())
	active, legacy := modelusage.RecoveryDir(), t.TempDir()
	now := time.Now().UTC()
	e := modelusage.Event{Purpose: "image", Operation: "images/edits", Subject: "fixture", Endpoint: "https://fixture.invalid/v1", Model: "art", Started: now, Finished: &now, Outcome: "ok", Usage: json.RawMessage(`{"total_tokens":5}`)}
	for _, dir := range []string{active, legacy} {
		e.ID = modelusage.NewID()
		if err := modelusage.Deliver(ctx, nil, dir, e); err != nil {
			t.Fatal(err)
		}
	}
	e.ID = modelusage.NewID()
	var acks atomic.Int32
	agent := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case "GET /internal/model-usage":
			json.NewEncoder(w).Encode([]modelusage.Delivery{{ID: "pi-event", Event: e}})
		case "POST /internal/model-usage/ack":
			acks.Add(1)
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer agent.Close()
	missing := filepath.Join(legacy, "missing")
	for i, tc := range []struct {
		lane, old string
		rows      int
	}{
		{"research", "", 1},      // Only the Pi receipt, even with files in the active outbox.
		{"media", legacy, 1},     // Same env_file and archive as research.
		{"research", legacy, 3},  // Both file outboxes are now consumed.
		{"research", missing, 3}, // Report the bad path and still acknowledge Pi.
	} {
		t.Setenv(modelusage.LegacyOutboxEnv, tc.old)
		err := syncLaneUsage(ctx, db, tc.lane, agent.URL)
		if tc.old == missing {
			if !errors.Is(err, os.ErrNotExist) || !strings.Contains(err.Error(), missing) {
				t.Fatalf("missing legacy outbox: %v", err)
			}
		} else if err != nil {
			t.Fatal(err)
		}
		if got := acks.Load(); got != int32(i+1) {
			t.Fatalf("%s: %d acknowledged batches after %d cycles", tc.lane, got, i+1)
		}
		var rows, tokens int
		if err := db.QueryRow(ctx, sqltest.ReceiptTotals).Scan(&rows, &tokens); err != nil || rows != tc.rows || tokens != 5*tc.rows {
			t.Fatalf("%s: got %d rows, %d tokens, %v", tc.lane, rows, tokens, err)
		}
	}
}
