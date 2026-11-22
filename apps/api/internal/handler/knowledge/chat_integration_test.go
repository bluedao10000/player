package knowledge_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	q "anime-agent/api/internal/repository/queries/knowledge"
	routing "anime-agent/api/internal/router"
	domain "anime-agent/api/internal/service/knowledge"
	"anime-agent/api/internal/testsupport"
	fixtures "anime-agent/api/internal/testsupport/fixtures"
)

func TestWorkbenchChatConfirmation(t *testing.T) {
	ctx := context.Background()
	db := testsupport.Database(t)
	if err := domain.Sweep(ctx, db, "tidepost-original", true, false); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, fixtures.SetChatEditionLength); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WORKBENCH_LOCAL_ENABLED", "true")
	pi := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Question string
			Tools    map[string]json.RawMessage
			Prior    []json.RawMessage
		}
		if json.NewDecoder(r.Body).Decode(&payload) != nil || len(payload.Tools["get_workspace_context"]) == 0 {
			w.WriteHeader(400)
			return
		}
		patch := map[string]any{}
		switch payload.Question {
		case "budget":
			patch = map[string]any{"tool": "update_token_budget", "token_limit": 1000000}
		case "overwrite":
			patch = map[string]any{"tool": "update_production_policy", "preserve_published": false}
		case "scope":
			patch = map[string]any{"tool": "update_episode_scope", "first_episode": 1, "last_episode": 20}
		case "retry":
			patch = map[string]any{"tool": "retry_episode", "episode": 9}
		case "pause":
			patch = map[string]any{"tool": "pause_production"}
		case "resume":
			patch = map[string]any{"tool": "resume_production"}
		case "policy":
			patch = map[string]any{"tool": "update_production_policy", "character_consistency": "strict"}
		default:
			if len(payload.Prior) != 0 {
				t.Error("first turn unexpectedly has history")
			}
			patch = nil
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"text": "来自工作台数据", "patch": patch})
	}))
	defer pi.Close()
	t.Setenv("KNOWLEDGE_RESEARCH_URL", pi.URL)
	router := gin.New()
	routing.Register(router, db)
	token := strings.Repeat("a", 64)
	digest := sha256.Sum256([]byte(token))
	if _, err := db.Exec(ctx, q.CreateEditorSession, digest[:], "email:fixture@example.test"); err != nil {
		t.Fatal(err)
	}
	request := func(method, path, auth string, data any, want int) []byte {
		t.Helper()
		payload, _ := json.Marshal(data)
		req := httptest.NewRequest(method, "/api/v1/editor/"+path, bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+auth)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != want {
			t.Fatalf("%s %s got %d, want %d: %s", method, path, w.Code, want, w.Body)
		}
		return w.Body.Bytes()
	}
	plan := "a01c3782-97fc-43ac-bb47-5875974775cb"
	request("POST", "plans", token, map[string]any{"id": plan, "world": "tidepost", "first": 1, "last": 1, "budget": 60000, "instruction": "fixture"}, 204)
	turn := func(n int, question string) string {
		id := fmt.Sprintf("a01c3782-97fc-43ac-bb47-%012d", n)
		body := map[string]any{"request_id": id, "world": "tidepost", "plan_id": plan, "text": question}
		request("POST", "chat", token, body, 204)
		return "chat/" + id + "/resolve"
	}
	state := func(wantBudget int64, wantRevision int, wantPaused bool) {
		t.Helper()
		var result struct {
			Plans []struct {
				Budget   int64 `json:"token_limit"`
				Revision int
				Paused   bool `json:"operator_paused"`
			}
		}
		if err := json.Unmarshal(request("GET", "plans", token, nil, 200), &result); err != nil {
			t.Fatal(err)
		}
		p := result.Plans[0]
		if p.Budget != wantBudget || p.Revision != wantRevision || p.Paused != wantPaused {
			t.Fatalf("unexpected plan: %+v", p)
		}
	}
	request("GET", "chat", "", nil, 401)
	turn(1, "query")
	first := turn(2, "budget")
	stale := turn(3, "pause")
	state(60000, 1, false)
	history := request("GET", "chat", token, nil, 200)
	if !bytes.Contains(history, []byte(`"token_limit": [60000, 1000000]`)) && !bytes.Contains(history, []byte(`"token_limit":[60000,1000000]`)) {
		t.Fatalf("missing exact diff: %s", history)
	}
	confirm := map[string]bool{"confirm": true}
	request("POST", first, token, confirm, 204)
	request("POST", first, token, confirm, 204)
	request("POST", stale, token, confirm, 409)
	state(1000000, 2, false)
	request("POST", turn(4, "pause"), token, confirm, 204)
	state(1000000, 3, true)
	request("POST", turn(5, "resume"), token, confirm, 204)
	state(1000000, 4, false)
	request("POST", turn(6, "policy"), token, map[string]bool{"confirm": false}, 204)
	state(1000000, 4, false)
	request("POST", turn(7, "policy"), token, confirm, 204)
	state(1000000, 5, false)
	request("POST", turn(8, "scope"), token, confirm, 204)
	state(1000000, 6, false)
	var result struct {
		Plans []struct {
			Last  int `json:"last_episode"`
			Stage string
		}
	}
	if err := json.Unmarshal(request("GET", "plans", token, nil, 200), &result); err != nil {
		t.Fatal(err)
	}
	if result.Plans[0].Last != 20 || result.Plans[0].Stage != "inventory" {
		t.Fatal("scope update didn't queue inventory")
	}
	if _, err := db.Exec(ctx, fixtures.SeedChatRetry, plan); err != nil {
		t.Fatal(err)
	}
	request("POST", turn(9, "retry"), token, confirm, 204)
	var policy, count int
	if err := db.QueryRow(ctx, fixtures.ChatRetryPolicy).Scan(&policy); err != nil || policy != 2 {
		t.Fatalf("retry was not queued: policy %d, %v", policy, err)
	}
	request("POST", turn(10, "overwrite"), token, confirm, 409)
	state(1000000, 7, false)
	if err := db.QueryRow(ctx, fixtures.ChatOperationCount, plan).Scan(&count); err != nil || count != 6 {
		t.Fatalf("wrong audit count %d: %v", count, err)
	}
}
