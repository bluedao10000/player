package knowledge_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	routing "anime-agent/api/internal/router"
	"anime-agent/api/internal/testsupport"
	sqltest "anime-agent/api/internal/testsupport/fixtures"
)

func TestReaderConsentSessionAndParagraphs(t *testing.T) {
	db := testsupport.Database(t)
	r := gin.New()
	routing.Register(r, db)
	ctx := context.Background()
	seedCode := func(age string, attempts int) {
		t.Helper()
		hash, err := bcrypt.GenerateFromPassword([]byte("123456"), bcrypt.MinCost)
		if err != nil {
			t.Fatal(err)
		}
		_, err = db.Exec(ctx, sqltest.SeedEmailCode, hash, age, attempts)
		if err != nil {
			t.Fatal(err)
		}
	}
	var token, id string
	call := func(method, path string, body any, status int) map[string]any {
		t.Helper()
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(method, "/api/v1/reader/"+path, bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("X-Reader-ID", id)
		res := httptest.NewRecorder()
		r.ServeHTTP(res, req)
		if res.Code != status {
			t.Fatalf("%s got %d, want %d", path, res.Code, status)
		}
		var out map[string]any
		if res.Code == 200 {
			if err := json.Unmarshal(res.Body.Bytes(), &out); err != nil {
				t.Fatal(err)
			}
		}
		return out
	}
	verify := gin.H{"email": "Test@Example.org", "code": "123456"}
	seedCode("6 min", 0)
	call("POST", "verify", verify, 400)
	seedCode("0 sec", 4)
	call("POST", "verify", gin.H{"email": "test@example.org", "code": "000000"}, 400)
	call("POST", "verify", verify, 400)
	seedCode("0 sec", 0)
	token = call("POST", "verify", verify, 200)["token"].(string)
	call("POST", "verify", verify, 400)
	state := call("GET", "state", nil, 200)
	if state["pending"] != true {
		t.Fatal("expected pending import")
	}
	id = state["id"].(string)
	item := gin.H{"href": "/story/last-ferry", "title": "末班渡船", "work": "潮汐邮局", "anchor": "reading-3", "percent": 80, "recordedAt": 100}
	imported := gin.H{"choice": "merge", "bookmarks": []any{item, item}, "positions": []any{item}}
	call("POST", "merge", imported, 204)
	item["anchor"] = "reading-2"
	item["percent"] = 30
	item["recordedAt"] = 200
	call("POST", "progress", item, 204)
	item["anchor"] = "reading-1"
	item["recordedAt"] = 90
	call("POST", "progress", item, 204)
	item["href"] = "/story/night-courier"
	call("POST", "merge", imported, 204)
	state = call("GET", "state", nil, 200)
	saved := state["bookmarks"].([]any)
	positions := state["positions"].([]any)
	if state["pending"] != false || len(saved) != 1 || len(positions) != 1 || positions[0].(map[string]any)["anchor"] != "reading-2" {
		t.Fatal("import/retry did not preserve latest paragraph")
	}
	id = "stale"
	call("POST", "progress", item, 409)
	id = state["id"].(string)
	call("POST", "logout", gin.H{}, 204)
	call("GET", "state", nil, 401)
}
