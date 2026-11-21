package knowledge_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	routing "anime-agent/api/internal/router"
	"anime-agent/api/internal/testsupport"
	sqltest "anime-agent/api/internal/testsupport/fixtures"
)

func TestNativeReaderLoginAndLink(t *testing.T) {
	db := testsupport.Database(t)
	router := gin.New()
	routing.Register(router, db)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	previous := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = previous })
	t.Setenv("WECHAT_IOS_APP_ID", "wx-fixture-mobile")
	t.Setenv("WECHAT_IOS_APP_SECRET", "fixture-secret")
	http.DefaultTransport = wechatRoundTrip(func(r *http.Request) (*http.Response, error) {
		var body string
		switch r.URL.Host {
		case "appleid.apple.com":
			body = `{"keys":[{"kty":"RSA","alg":"RS256","use":"sig","kid":"native-fixture","e":"AQAB","n":"` + base64.RawURLEncoding.EncodeToString(key.N.Bytes()) + `"}]}`
		case "api.weixin.qq.com":
			if r.URL.Query().Get("appid") != "wx-fixture-mobile" {
				t.Fatal("wrong mobile application")
			}
			body = `{"openid":"mobile-open-id","unionid":"same-union","access_token":"fixture"}`
		default:
			t.Fatal("unexpected provider")
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	var token, id string
	call := func(method, path string, body any, want int) map[string]any {
		t.Helper()
		b, _ := json.Marshal(body)
		req := httptest.NewRequest(method, "/api/v1/reader/"+path, bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("X-Reader-ID", id)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != want {
			t.Fatalf("%s got %d: %s", path, w.Code, w.Body.String())
		}
		var result map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &result)
		return result
	}
	nonce := call("GET", "native/challenge", nil, 200)["nonce"].(string)
	hash := sha256.Sum256([]byte(nonce))
	claims := jwt.MapClaims{"iss": "https://appleid.apple.com", "sub": "apple-user", "aud": "com.kmszz.starWiki", "exp": time.Now().Add(time.Minute).Unix(), "nonce": hex.EncodeToString(hash[:])}
	j := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	j.Header["kid"] = "native-fixture"
	signed, _ := j.SignedString(key)
	payload := map[string]string{"nonce": nonce, "token": signed, "bundle": "com.kmszz.starWiki"}
	token = call("POST", "native/apple", payload, 200)["token"].(string)
	call("POST", "native/apple", payload, 401)
	state := call("GET", "state", nil, 200)
	id = state["id"].(string)
	if state["appleLinked"] != true {
		t.Fatal(state)
	}
	call("POST", "native/link/wechat", map[string]string{"code": "ok"}, 200)
	token = call("POST", "native/wechat", map[string]string{"code": "ok"}, 200)["token"].(string)
	state = call("GET", "state", nil, 200)
	if state["id"] != id || state["mobileWechatLinked"] != true {
		t.Fatal(state)
	}
	// The website can resolve the verified mobile UnionID on its next OAuth exchange.
	var union string
	if err := db.QueryRow(context.Background(), sqltest.ReaderByUnionID).Scan(&union); err != nil || union != id {
		t.Fatal(err)
	}
}
