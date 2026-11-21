package knowledge_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	routing "anime-agent/api/internal/router"
	domain "anime-agent/api/internal/service/knowledge"
	"anime-agent/api/internal/testsupport"
	sqltest "anime-agent/database/queries/fixtures"
)

func TestEditorialReviewAndPublication(t *testing.T) {
	ctx := context.Background()
	db := testsupport.Database(t)
	if err := domain.Sweep(ctx, db, "tidepost-original", true, false); err != nil {
		t.Fatal(err)
	}
	admin := strings.Repeat("t", 40)
	t.Setenv("KNOWLEDGE_ADMIN_TOKEN", admin)
	t.Setenv("KNOWLEDGE_ADMIN_OPERATOR", "test-editor")
	r := gin.New()
	routing.Register(r, db)
	call := func(method, path, token string, payload any, want int) []byte {
		t.Helper()
		body, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(method, "/api/v1/"+path, bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != want {
			t.Fatalf("%s %s: %d: %s", method, path, w.Code, w.Body.String())
		}
		return w.Body.Bytes()
	}
	obj := func(raw []byte) map[string]string {
		t.Helper()
		var v map[string]string
		if err := json.Unmarshal(raw, &v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	session := obj(call("POST", "admin/knowledge/editor/session", admin, nil, 200))["token"]
	create := func(key string) string {
		return obj(call("POST", "editor/tasks", session, gin.H{"world": "tidepost", "chapter": "night-courier", "instruction": "修订邮筒发现的正文和场景", "request_id": key}, 200))["id"]
	}
	id := create("98b5fb50-a85e-48ec-a00e-746c2cdd45c2")
	if create("98b5fb50-a85e-48ec-a00e-746c2cdd45c2") != id {
		t.Fatal("duplicate submission")
	}
	call("POST", "editor/tasks", session, gin.H{"world": "other", "chapter": "night-courier", "instruction": "补充", "request_id": "beeb2d2a-4ea6-4eed-b7c8-a5a19c097f9b"}, 400)
	second := create("8c61ec83-fa29-444c-87bf-0b907ea87964")
	task := "editor/tasks/" + id
	decision := func(action string, round, attempt int, note string, code int) {
		call("POST", task+"/"+action, session, gin.H{"round": round, "attempt": attempt, "note": note}, code)
	}
	decision("approve", 1, 1, "", 409)
	chapter := "knowledge/worlds/tidepost/chapters/night-courier"
	scene := "knowledge/worlds/tidepost/scenes/unstamped-letter"
	initialCh := call("GET", chapter, "", nil, 200)
	initialScene := call("GET", scene, "", nil, 200)
	const revision = "阿澪在红邮筒里找到了一封信。"
	model := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		var input struct{ Sources []struct{ ID, Text string } }
		if err := json.NewDecoder(req.Body).Decode(&input); err != nil || len(input.Sources) == 0 {
			http.Error(w, "no evidence", 400)
			return
		}
		s := input.Sources[0]
		if strings.HasSuffix(req.URL.Path, "/drafting") {
			json.NewEncoder(w).Encode([]gin.H{{"key": "block/mailbox", "text": revision, "source": s.ID, "quote": s.Text}, {"key": "scene/unstamped-letter", "text": revision, "source": s.ID, "quote": s.Text}})
		} else if strings.Contains(req.URL.Path, "/1/1/") {
			json.NewEncoder(w).Encode(gin.H{"pass": false, "note": "缩短场景说明"})
		} else {
			json.NewEncoder(w).Encode(gin.H{"pass": true, "note": "已核对原创定稿"})
		}
	}))
	defer model.Close()
	drain := func() {
		t.Helper()
		for i := 0; i < 32; i++ {
			var count int
			if err := db.QueryRow(ctx, sqltest.CountActiveEditorTasks).Scan(&count); err != nil {
				t.Fatal(err)
			}
			if count == 0 {
				return
			}
			if err := domain.ProcessEditorial(ctx, db, model.URL); err != nil {
				t.Fatal(err)
			}
		}
		t.Fatal("editor did not settle")
	}
	drain()
	call("GET", task+"/preview?round=1&attempt=2", "", nil, 401)
	if !bytes.Equal(initialCh, call("GET", chapter, "", nil, 200)) || !bytes.Equal(initialScene, call("GET", scene, "", nil, 200)) {
		t.Fatal("review changed public pages")
	}
	if !bytes.Contains(call("GET", task+"/preview?round=1&attempt=2", session, nil, 200), []byte(revision)) {
		t.Fatal("preview missing updated copy")
	}
	decision("return", 1, 2, "再精简一些", 204)
	call("GET", task+"/preview?round=1&attempt=2", session, nil, 404)
	drain()
	decision("approve", 1, 2, "", 409)
	decision("approve", 2, 1, "", 204)
	decision("approve", 2, 1, "", 204)
	currentCh := call("GET", chapter, "", nil, 200)
	currentScene := call("GET", scene, "", nil, 200)
	if !bytes.Contains(currentCh, []byte(revision)) || !bytes.Contains(currentScene, []byte(revision)) || !bytes.Contains(currentCh, []byte("passage-unstamped-letter")) || !bytes.Contains(currentScene, []byte("/story/night-courier#passage-unstamped-letter")) {
		t.Fatal("release lost text or anchor")
	}
	task = "editor/tasks/" + second
	decision("approve", 1, 2, "", 409)
	var status string
	if err := db.QueryRow(ctx, sqltest.EditorStatus, second).Scan(&status); err != nil || status != "conflict" {
		t.Fatal("stale task overwrote release", status, err)
	}
	call("POST", "editor/logout", session, nil, 204)
	call("GET", "editor/tasks", session, nil, 401)
}
