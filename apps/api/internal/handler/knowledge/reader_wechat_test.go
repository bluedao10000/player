package knowledge_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	routing "anime-agent/api/internal/router"
	"anime-agent/api/internal/testsupport"
	sqltest "anime-agent/api/internal/testsupport/fixtures"
)

type wechatRoundTrip func(*http.Request) (*http.Response, error)

func (f wechatRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestWechatReaderAndMembership(t *testing.T) {
	db := testsupport.Database(t)
	r := gin.New()
	routing.Register(r, db)
	ctx := context.Background()
	t.Setenv("WECHAT_APP_ID", "fixture-app")
	t.Setenv("WECHAT_APP_SECRET", "fixture-secret")
	t.Setenv("WECHAT_CALLBACK_URL", "https://example.test/reader-auth/callback/wechat")
	admin := strings.Repeat("x", 32)
	t.Setenv("KNOWLEDGE_ADMIN_TOKEN", admin)
	t.Setenv("KNOWLEDGE_ADMIN_OPERATOR", "test")
	previous := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = previous })
	http.DefaultTransport = wechatRoundTrip(func(req *http.Request) (*http.Response, error) {
		if req.URL.Host != "api.weixin.qq.com" || req.URL.Path != "/sns/oauth2/access_token" || req.URL.Query().Get("secret") != "fixture-secret" {
			t.Fatal("unexpected provider request")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"access_token":"fixture-token","openid":"wx-fixture"}`))}, nil
	})
	var token, id string
	call := func(method, path string, data any, want int) map[string]any {
		t.Helper()
		body, _ := json.Marshal(data)
		req := httptest.NewRequest(method, "/api/v1/"+path, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("X-Reader-ID", id)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != want {
			t.Fatalf("%s: got %d want %d", path, w.Code, want)
		}
		var out map[string]any
		if w.Code == 200 {
			if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
				t.Fatal(err)
			}
		}
		return out
	}
	flow := func(path string) gin.H {
		data := call("POST", "reader/wechat/"+path, gin.H{"returnTo": "/story/last-ferry#resume"}, 200)
		u, err := url.Parse(data["url"].(string))
		if err != nil || u.Host != "open.weixin.qq.com" || u.Query().Get("redirect_uri") != "https://example.test/reader-auth/callback/wechat" {
			t.Fatal("unexpected QR request")
		}
		return gin.H{"state": u.Query().Get("state"), "proof": data["proof"], "code": "fixture"}
	}
	first := flow("start")
	call("POST", "reader/wechat/finish", gin.H{"state": first["state"], "proof": strings.Repeat("0", 64), "code": "fixture"}, 400)
	result := call("POST", "reader/wechat/finish", first, 200)
	token = result["token"].(string)
	call("POST", "reader/wechat/finish", first, 400)
	me := call("GET", "reader/state", nil, 200)
	id = me["id"].(string)
	if me["email"] != nil || me["wechatLinked"] != true || me["plan"] != "free" || result["returnTo"] != "/story/last-ferry#resume" {
		t.Fatal("incorrect new reader")
	}
	call("POST", "reader/logout", gin.H{}, 204)
	token = ""
	token = call("POST", "reader/wechat/finish", flow("start"), 200)["token"].(string)
	if call("GET", "reader/state", nil, 200)["id"] != id {
		t.Fatal("repeat login created another reader")
	}
	savedToken := token
	planPath := "admin/knowledge/readers/" + id + "/plan"
	call("PUT", planPath, gin.H{"plan": "pro"}, 401)
	token = admin
	until := time.Now().Add(time.Hour)
	for _, plan := range []string{"plus", "pro", "pro"} {
		call("PUT", planPath, gin.H{"plan": plan, "validUntil": until}, 204)
	}
	token = savedToken
	me = call("GET", "reader/state", nil, 200)
	if me["plan"] != "pro" {
		t.Fatal("pro grant not effective")
	}
	var count int
	if err := db.QueryRow(ctx, sqltest.CountPlanChanges, id).Scan(&count); err != nil || count != 2 {
		t.Fatal("expected two actual changes")
	}
	if _, err := db.Exec(ctx, sqltest.ExpireMembership, id); err != nil {
		t.Fatal(err)
	}
	if call("GET", "reader/state", nil, 200)["plan"] != "free" {
		t.Fatal("expired grant still active")
	}
	item := gin.H{"href": "/story/night-courier", "title": "a", "work": "b", "saved": true}
	call("POST", "reader/bookmark", item, 204)
	call("POST", "reader/bookmark", item, 204)
	item["href"] = "/story/last-ferry"
	call("POST", "reader/bookmark", item, 204)
	call("POST", "reader/merge", gin.H{"choice": "merge", "bookmarks": []any{item}}, 204)
	if call("GET", "reader/state", nil, 200)["pending"] != false {
		t.Fatal("import did not persist consent")
	}
	bound := flow("link")
	result = call("POST", "reader/wechat/finish", bound, 200)
	if result["token"] != "" {
		t.Fatal("link rotated session unexpectedly")
	}
	// Prove a second login method before attaching it. Later email login uses the
	// same reader ID, which owns the previously saved chapter.
	hash, err := bcrypt.GenerateFromPassword([]byte("123456"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	otp := gin.H{"email": "linked@example.test", "code": "123456"}
	seed := func(email string) {
		t.Helper()
		_, err := db.Exec(ctx, sqltest.ResetEmailCode, email, hash)
		if err != nil {
			t.Fatal(err)
		}
	}
	seed("linked@example.test")
	call("POST", "reader/email/link", otp, 204)
	seed("linked@example.test")
	token = call("POST", "reader/verify", otp, 200)["token"].(string)
	same := call("GET", "reader/state", nil, 200)
	if same["id"] != id || len(same["bookmarks"].([]any)) != 2 {
		t.Fatal("email login lost the WeChat reader's shelf")
	}
	seed("another@example.test")
	token = call("POST", "reader/verify", gin.H{"code": "123456", "email": "another@example.test"}, 200)["token"].(string)
	other := call("GET", "reader/state", nil, 200)
	id = other["id"].(string)
	call("POST", "reader/wechat/finish", flow("link"), 409)
	if call("GET", "reader/state", nil, 200)["wechatLinked"] != false {
		t.Fatal("conflicting link changed other reader")
	}

}
