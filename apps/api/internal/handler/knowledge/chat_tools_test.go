package knowledge_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	q "anime-agent/api/internal/repository/queries/knowledge"
	routing "anime-agent/api/internal/router"
	"anime-agent/api/internal/testsupport"
	fixtures "anime-agent/api/internal/testsupport/fixtures"
)

func TestWorkbenchReadTools(t *testing.T) {
	ctx := context.Background()
	db := testsupport.Database(t)
	for _, sql := range []string{
		fixtures.SeedOpsSyncRuns, fixtures.SeedOpsWorlds, fixtures.SeedOpsProductionRequests, fixtures.SeedOpsEpisodeRuns,
		fixtures.SeedOpsEditors, fixtures.ChatToolFixtures["SeedChatToolCatalogExtras"], fixtures.ChatToolFixtures["SeedChatToolDetail"],
	} {
		if _, err := db.Exec(ctx, sql); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := db.Query(ctx, fixtures.ChatToolFixtures["ChatToolCatalogIDs"])
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]string{}
	for rows.Next() {
		var id, title string
		if err := rows.Scan(&id, &title); err != nil {
			t.Fatal(err)
		}
		ids[title] = id
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	var task string
	if err := db.QueryRow(ctx, fixtures.ChatToolFixtures["ChatToolEditorTask"]).Scan(&task); err != nil {
		t.Fatal(err)
	}
	token := strings.Repeat("b", 64)
	digest := sha256.Sum256([]byte(token))
	if _, err := db.Exec(ctx, q.CreateEditorSession, digest[:], "email:tools@example.test"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WORKBENCH_LOCAL_ENABLED", "true")

	type input struct {
		Question     string
		Tools        map[string]json.RawMessage
		Capabilities []string
		Prior        []struct{ Prompt, Reply string }
		AllowReads   bool `json:"allow_reads"`
		Reads        []struct {
			Name, Error string
			Data        json.RawMessage
		} `json:"read_results"`
	}
	incoming := make(chan input, 16)
	calls := func(name string, args map[string]string) map[string]any {
		return map[string]any{"name": name, "arguments": args}
	}
	replyCalls := func(calls ...map[string]any) map[string]any {
		return map[string]any{"read_calls": calls}
	}
	search := func(query string) map[string]any { return calls("search_anime_works", map[string]string{"q": query}) }
	selected := func(value string) map[string]any {
		return calls("read_selected_work", map[string]string{"work": value})
	}
	bebop := ids["Ops Bebop"]
	pi := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body input
		if json.NewDecoder(r.Body).Decode(&body) != nil {
			w.WriteHeader(400)
			return
		}
		if strings.Contains(r.URL.Path, "/1/2/") {
			incoming <- body
			_ = json.NewEncoder(w).Encode(map[string]any{"text": "本轮读取完成。"})
			return
		}
		var reply any = map[string]any{"text": "现有快照。"}
		switch body.Question {
		case "catalog":
			reply = replyCalls(search("工具测试"), calls("read_anime_work", map[string]string{"id": bebop}), search("no-such-catalog"))
		case "not publicly visible":
			reply = replyCalls(calls("read_anime_work", map[string]string{"id": ids["工具测试 · 隐藏目录"]}))
		case "selected":
			reply = replyCalls(selected(bebop))
		case "wrong model scope":
			reply = replyCalls(selected(ids["Ops Tidepost"]), calls("get_workspace_context", nil), calls("unknown_private", nil))
		case "without selection":
			reply = replyCalls(selected(bebop))
		case "too many calls":
			reply = replyCalls(search("a"), search("b"), search("c"), search("d"))
		case "batch too large":
			detail := calls("read_anime_work", map[string]string{"id": ids["工具测试 · 公开目录 2"]})
			reply = replyCalls(detail, detail, detail)
		case "too large":
			reply = replyCalls(calls("read_anime_work", map[string]string{"id": ids["工具测试 · 公开目录 1"]}))
		case "timeout":
			reply = replyCalls(search("工具测试"))
		case "cross write":
			reply = map[string]any{"text": "修改计划", "patch": map[string]any{"tool": "update_production_plan", "world": "cowboy-bebop-tv", "token_limit": 200000}}
		case "history scope":
			incoming <- body
		}
		_ = json.NewEncoder(w).Encode(reply)
	}))
	defer pi.Close()
	t.Setenv("KNOWLEDGE_RESEARCH_URL", pi.URL)
	router := gin.New()
	routing.Register(router, db)
	request := func(method, path string, body any, want int, authenticate bool) []byte {
		t.Helper()
		data, _ := json.Marshal(body)
		r := httptest.NewRequest(method, path, bytes.NewReader(data))
		r.Header.Set("Content-Type", "application/json")
		if authenticate {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		out := httptest.NewRecorder()
		router.ServeHTTP(out, r)
		if out.Code != want {
			t.Fatalf("%s %s: status=%d want=%d: %s", method, path, out.Code, want, out.Body)
		}
		return out.Body.Bytes()
	}
	n := 0
	turn := func(world, question string, selection []map[string]any, status int) map[string]any {
		t.Helper()
		n++
		body := map[string]any{"request_id": fmt.Sprintf("10000000-0000-4000-8000-%012d", n), "world": world, "text": question, "selected_works": selection}
		request("POST", "/api/v1/editor/chat", body, status, true)
		return body
	}
	next := func() input {
		t.Helper()
		select {
		case body := <-incoming:
			return body
		default:
			t.Fatal("model follow-up missing")
			return input{}
		}
	}
	choice := []map[string]any{{"work": "cowboy-bebop-tv"}}
	t.Run("public catalog contract", func(t *testing.T) {
		turn("tidepost", "catalog", nil, 204)
		body := next()
		wantSnapshots := []string{"get_workspace_context", "get_knowledge_completeness", "get_production_plan", "get_episode_status", "get_media_candidates", "get_execution_history"}
		if len(body.Tools) != len(wantSnapshots) || body.AllowReads || len(body.Reads) != 3 {
			t.Fatalf("invalid model input %+v", body)
		}
		for _, name := range wantSnapshots {
			if body.Tools[name] == nil {
				t.Fatalf("snapshot missing %s", name)
			}
		}
		var workspace map[string]json.RawMessage
		if err := json.Unmarshal(body.Tools["get_workspace_context"], &workspace); err != nil || len(workspace) != 4 || workspace["governance"] == nil {
			t.Fatalf("workspace %s", body.Tools["get_workspace_context"])
		}
		var items []struct{ ID string }
		if err := json.Unmarshal(body.Reads[0].Data, &items); err != nil || len(items) != 8 {
			t.Fatalf("search data: %s", body.Reads[0].Data)
		}
		want := request("GET", "/api/v1/knowledge/works?q="+url.QueryEscape("工具测试")+"&limit=8", nil, 200, false)
		var page struct{ Items json.RawMessage }
		_ = json.Unmarshal(want, &page)
		var toolItems, publicItems []map[string]any
		if err := json.Unmarshal(body.Reads[0].Data, &toolItems); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(page.Items, &publicItems); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(toolItems, publicItems) || string(body.Reads[2].Data) != "[]" {
			t.Fatalf("public list incompatible: %s; empty: %s", body.Reads[0].Data, body.Reads[2].Data)
		}
		var detail, publicDetail map[string]any
		_ = json.Unmarshal(body.Reads[1].Data, &detail)
		_ = json.Unmarshal(request("GET", "/api/v1/knowledge/works/"+bebop, nil, 200, false), &publicDetail)
		for _, key := range []string{"id", "canonical_title", "type", "season_year", "episodes", "status", "studios", "provenance"} {
			if !reflect.DeepEqual(detail[key], publicDetail[key]) {
				t.Fatalf("detail field %s differs", key)
			}
		}
		if len(detail) != 11 || len(detail["titles"].([]any)) != 20 || detail["sources"] != nil || detail["related"] == nil {
			t.Fatalf("detail incompatible: %v", detail)
		}
		turn("tidepost", "not publicly visible", nil, 204)
		if code := next().Reads[0].Error; code != "not_found" {
			t.Fatalf("hidden catalog detail error %q", code)
		}
	})
	t.Run("explicit selection authorization", func(t *testing.T) {
		request("POST", "/api/v1/editor/chat", map[string]any{"selected_works": choice}, 401, false)
		saved := turn("tidepost", "selected", choice, 204)
		got := next()
		if len(got.Reads) != 1 || got.Reads[0].Error != "" || !bytes.Contains(got.Reads[0].Data, []byte(bebop)) || !bytes.Contains(got.Reads[0].Data, []byte("cowboy-bebop-tv")) {
			t.Fatalf("selected result = %+v", got.Reads)
		}
		// The same explicit selection may be repeated by UUID. Removing it changes the request.
		saved["selected_works"] = []map[string]any{{"work": bebop}}
		request("POST", "/api/v1/editor/chat", saved, 204, true)
		saved["selected_works"] = nil
		request("POST", "/api/v1/editor/chat", saved, 409, true)
		turn("tidepost", "without selection", nil, 204)
		denied := next()
		if denied.Reads[0].Error != "work_not_selected" || strings.Contains(strings.Join(denied.Capabilities, ","), "read_selected_work") {
			t.Fatalf("unselected result %+v", denied)
		}
		turn("tidepost", "wrong model scope", choice, 204)
		wrong := next().Reads
		if wrong[0].Error != "work_not_selected" || wrong[1].Error != "tool_not_available" || wrong[2].Error != "tool_not_available" {
			t.Fatalf("model scope failure = %+v", wrong)
		}
		turn("tidepost", "history scope", nil, 204)
		for _, old := range next().Prior {
			if old.Prompt == "selected" || old.Prompt == "wrong model scope" {
				t.Fatalf("selected data leaked to next turn: %+v", old)
			}
		}
		raw := request("GET", "/api/v1/editor/chat", nil, 200, true)
		if !bytes.Contains(raw, []byte("get_execution_history")) || !bytes.Contains(raw, []byte("read_selected_work")) {
			t.Fatal("tool receipts unavailable in history")
		}
	})
	t.Run("reject invalid selections", func(t *testing.T) {
		turn("tidepost", "never called", []map[string]any{{"work": bebop}, {"work": "tidepost"}}, 400)
		turn("tidepost", "never called", []map[string]any{{"work": "missing-world"}}, 404)
		for _, args := range []map[string]any{
			{"work": bebop, "plan_id": "00000000-0000-4000-8000-000000000101"},
			{"work": bebop, "task_id": task},
			{"work": bebop, "episode": 901},
			{"work": "tidepost"},
		} {
			turn("tidepost", "never called", []map[string]any{args}, 400)
		}
		turn("cowboy-bebop-tv", "history scope", []map[string]any{{"work": "tidepost", "plan_id": "00000000-0000-4000-8000-000000000101", "task_id": task, "episode": 901}}, 204)
		next()
	})
	t.Run("writes stay with current world", func(t *testing.T) {
		n++
		id := fmt.Sprintf("10000000-0000-4000-8000-%012d", n)
		request("POST", "/api/v1/editor/chat", map[string]any{
			"request_id": id, "world": "tidepost", "text": "cross write", "selected_works": choice,
			"plan_id": "00000000-0000-4000-8000-000000000101",
		}, 204, true)
		var history struct {
			Turns []struct {
				ID, State, Reply string
				Proposal         *json.RawMessage
			}
		}
		raw := request("GET", "/api/v1/editor/chat", nil, 200, true)
		if err := json.Unmarshal(raw, &history); err != nil {
			t.Fatal(err)
		}
		last := history.Turns[len(history.Turns)-1]
		if last.ID != id || last.State != "answered" || last.Proposal != nil || !strings.Contains(last.Reply, "当前对话作品") {
			t.Fatalf("cross-world write was not rejected: %+v", last)
		}
	})
	t.Run("limits and timeouts", func(t *testing.T) {
		turn("tidepost", "too many calls", nil, 204)
		if code := next().Reads[0].Error; code != "call_limit" {
			t.Fatalf("call limit: %s", code)
		}
		if _, err := db.Exec(ctx, fixtures.ChatToolFixtures["SeedChatToolPayload"]); err != nil {
			t.Fatal(err)
		}
		turn("tidepost", "too large", nil, 204)
		if code := next().Reads[0].Error; code != "payload_limit" {
			t.Fatalf("payload limit: %s", code)
		}
		if _, err := db.Exec(ctx, fixtures.ChatToolFixtures["SeedChatToolBatchPayload"]); err != nil {
			t.Fatal(err)
		}
		turn("tidepost", "batch too large", nil, 204)
		batch := next().Reads
		if len(batch) != 3 || batch[0].Error != "" || batch[1].Error != "" || batch[2].Error != "payload_limit" {
			t.Fatal("shared byte budget not enforced")
		}
		tx, err := db.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		if _, err = tx.Exec(ctx, fixtures.ChatToolFixtures["LockChatToolCatalog"]); err != nil {
			t.Fatal(err)
		}
		turn("tidepost", "timeout", nil, 204)
		if code := next().Reads[0].Error; code != "timeout" {
			t.Fatalf("read timeout: %s", code)
		}
	})
}
