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
	if _, err := db.Exec(ctx, fixtures.SetChatEditionLength, 20); err != nil {
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
		case "create", "create other", "create public":
			patch = map[string]any{"tool": "create_production_plan", "first_episode": 1, "last_episode": 5, "token_limit": 300000, "instruction": "补齐前 5 集（本地验收）"}
			if payload.Question == "create other" {
				patch["world"] = "cowboy-bebop-tv"
			} else if payload.Question == "create public" {
				patch["publish_to_reader"] = true
			}
		case "change public":
			patch = map[string]any{"tool": "update_token_budget", "token_limit": 990000, "publish_to_reader": true}
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
	createID := "a01c3782-97fc-43ac-bb47-000000000011"
	request("POST", "chat", token, map[string]any{"request_id": createID, "world": "tidepost", "text": "create"}, 204)
	createHistory := request("GET", "chat", token, nil, 200)
	if !bytes.Contains(createHistory, []byte(`"publish_to_reader":false`)) && !bytes.Contains(createHistory, []byte(`"publish_to_reader": false`)) {
		t.Fatalf("create_production_plan diff missing local-only publish_to_reader=false: %s", createHistory)
	}
	request("POST", "chat/"+createID+"/resolve", token, confirm, 204)
	request("POST", "chat/"+createID+"/resolve", token, confirm, 204)
	var createdPlans struct {
		Plans []struct {
			ID     string         `json:"id"`
			First  int            `json:"first_episode"`
			Last   int            `json:"last_episode"`
			Budget int64          `json:"token_limit"`
			Policy map[string]any `json:"production_policy"`
		}
	}
	if err := json.Unmarshal(request("GET", "plans", token, nil, 200), &createdPlans); err != nil {
		t.Fatal(err)
	}
	if len(createdPlans.Plans) != 2 {
		t.Fatalf("expected 2 plans after idempotent create, got %d", len(createdPlans.Plans))
	}
	created := createdPlans.Plans[0]
	if created.First != 1 || created.Last != 5 || created.Budget != 300000 {
		t.Fatalf("created plan scope/budget mismatch: %+v", created)
	}
	if created.Policy["publish_to_reader"] != false || created.Policy["preserve_published"] != true || created.Policy["character_consistency"] != "standard" {
		t.Fatalf("created plan must default to local-only acceptance: %+v", created.Policy)
	}
	var chatState struct {
		Turns []struct {
			ID     string  `json:"id"`
			State  string  `json:"state"`
			PlanID *string `json:"plan_id"`
		} `json:"turns"`
	}
	if err := json.Unmarshal(request("GET", "chat", token, nil, 200), &chatState); err != nil {
		t.Fatal(err)
	}
	bound := false
	for _, row := range chatState.Turns {
		if row.ID == createID {
			bound = row.State == "applied" && row.PlanID != nil && *row.PlanID == created.ID && created.ID != plan
		}
	}
	if !bound {
		t.Fatalf("create turn was not bound to the new plan as applied: %+v", chatState.Turns)
	}

	editID := "a01c3782-97fc-43ac-bb47-000000000012"
	request("POST", "chat", token, map[string]any{"request_id": editID, "world": "tidepost", "plan_id": created.ID, "text": "policy"}, 204)
	request("POST", "chat/"+editID+"/resolve", token, confirm, 204)
	if err := json.Unmarshal(request("GET", "plans", token, nil, 200), &createdPlans); err != nil {
		t.Fatal(err)
	}
	edited := createdPlans.Plans[0]
	if edited.ID != created.ID || edited.Policy["publish_to_reader"] != false || edited.Policy["preserve_published"] != true || edited.Policy["character_consistency"] != "strict" {
		t.Fatalf("consistency update did not preserve the local-only policy: %+v", edited)
	}

	const worldError = "只能为当前对话的作品新建制作计划。"
	const episodesError = "请先登记作品总集数，再新建制作计划。"
	createTurn := func(n int, question string) string {
		t.Helper()
		id := fmt.Sprintf("a01c3782-97fc-43ac-bb47-%012d", n)
		request("POST", "chat", token, map[string]any{"request_id": id, "world": "tidepost", "text": question}, 204)
		return id
	}
	readCreateTurn := func(id string) (state, reply string, diff json.RawMessage) {
		t.Helper()
		var history struct {
			Turns []struct {
				ID, State, Reply string
				Diff             json.RawMessage
			}
		}
		if err := json.Unmarshal(request("GET", "chat", token, nil, 200), &history); err != nil {
			t.Fatal(err)
		}
		for _, row := range history.Turns {
			if row.ID == id {
				return row.State, row.Reply, row.Diff
			}
		}
		t.Fatalf("turn %s missing from history", id)
		return "", "", nil
	}
	noWrites := func(id string) {
		t.Helper()
		var plans struct{ Plans []json.RawMessage }
		if err := json.Unmarshal(request("GET", "plans", token, nil, 200), &plans); err != nil {
			t.Fatal(err)
		}
		if len(plans.Plans) != 2 {
			t.Fatalf("turn %s changed the plan count: %d", id, len(plans.Plans))
		}
		var audits int
		if err := db.QueryRow(ctx, fixtures.ChatTurnOperationCount, id).Scan(&audits); err != nil || audits != 0 {
			t.Fatalf("turn %s wrote %d audits: %v", id, audits, err)
		}
	}
	rejectedProposal := func(id, want string) {
		t.Helper()
		state, reply, diff := readCreateTurn(id)
		if state != "answered" || reply != want || string(diff) != "null" {
			t.Fatalf("turn %s got state=%q reply=%q diff=%s", id, state, reply, diff)
		}
		noWrites(id)
	}
	pendingProposal := func(id string) {
		t.Helper()
		state, reply, diff := readCreateTurn(id)
		if state != "pending" || len(diff) == 0 || string(diff) == "null" {
			t.Fatalf("turn %s got state=%q reply=%q diff=%s, want a pending proposal", id, state, reply, diff)
		}
	}
	rejectedConfirmation := func(id, want string) {
		t.Helper()
		var result struct{ Error string }
		if err := json.Unmarshal(request("POST", "chat/"+id+"/resolve", token, confirm, 409), &result); err != nil || result.Error != want {
			t.Fatalf("turn %s confirmation error=%q, want %q: %v", id, result.Error, want, err)
		}
		pendingProposal(id)
		noWrites(id)
	}

	const publishError = "工作台对话只支持本地验收计划，不能开启公网发布。"
	rejectedProposal(createTurn(17, "create public"), publishError)
	turn(18, "change public")
	rejectedProposal("a01c3782-97fc-43ac-bb47-000000000018", publishError)
	publicTamper := createTurn(19, "create")
	pendingProposal(publicTamper)
	if _, err := db.Exec(ctx, fixtures.ChangeChatProposalPublish, publicTamper); err != nil {
		t.Fatal(err)
	}
	rejectedConfirmation(publicTamper, publishError)

	rejectedProposal(createTurn(13, "create other"), worldError)
	tampered := createTurn(14, "create")
	pendingProposal(tampered)
	if _, err := db.Exec(ctx, fixtures.ChangeChatProposalWorld, tampered, "cowboy-bebop-tv"); err != nil {
		t.Fatal(err)
	}
	rejectedConfirmation(tampered, worldError)

	staleTotal := createTurn(15, "create")
	pendingProposal(staleTotal)
	if _, err := db.Exec(ctx, fixtures.SetChatEditionLength, nil); err != nil {
		t.Fatal(err)
	}
	rejectedConfirmation(staleTotal, episodesError)
	rejectedProposal(createTurn(16, "create"), episodesError)
}
