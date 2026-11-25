package knowledge_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"

	routing "anime-agent/api/internal/router"
	domain "anime-agent/api/internal/service/knowledge"
	"anime-agent/api/internal/testsupport"
	sqltest "anime-agent/api/internal/testsupport/fixtures"
)

func TestWorldReadingPath(t *testing.T) {
	db := testsupport.Database(t)
	ctx := context.Background()
	for range 2 {
		if err := domain.Sweep(ctx, db, "tidepost-original", true, false); err != nil {
			t.Fatal(err)
		}
	}
	r := gin.New()
	routing.Register(r, db)
	get := func(path string, status int) []byte {
		t.Helper()
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/knowledge/worlds/"+path, nil))
		if w.Code != status {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body.String())
		}
		return w.Body.Bytes()
	}
	var profile struct{ Appearances []struct{ Href string } }
	if err := json.Unmarshal(get("tidepost/characters/mio", 200), &profile); err != nil || len(profile.Appearances) != 4 {
		t.Fatalf("appearances: %v %v", profile, err)
	}
	var scene struct {
		Return struct {
			Href  string
			Block string `json:"block_key"`
		} `json:"return_to"`
	}
	if err := json.Unmarshal(get("tidepost/scenes/ferry-register", 200), &scene); err != nil || scene.Return.Href != "/story/last-ferry#passage-ferry-register" || scene.Return.Block != "missing-ticket" {
		t.Fatalf("return: %v %v", scene, err)
	}
	var v, revision int64
	if err := db.QueryRow(ctx, sqltest.WorldRevision).Scan(&v, &revision); err != nil || revision != 1 {
		t.Fatalf("reimport: %d %v", revision, err)
	}
	// Edit only the isolated test schema. Insert a preface and a second scene
	// pointing into the same paragraph. Both old links must still find that text.
	exec := func(sql string) {
		t.Helper()
		if _, e := db.Exec(ctx, sql, v); e != nil {
			t.Fatal(e)
		}
	}
	exec(sqltest.InsertPreface)
	exec(sqltest.InsertSecondScene)
	var chapter struct {
		Blocks []struct {
			Key, Text string
			Anchors   []string
		}
	}
	if e := json.Unmarshal(get("tidepost/chapters/last-ferry", 200), &chapter); e != nil {
		t.Fatal(e)
	}
	if len(chapter.Blocks) != 4 || chapter.Blocks[2].Key != "missing-ticket" || !strings.Contains(chapter.Blocks[2].Text, "船票少了一张") || strings.Join(chapter.Blocks[2].Anchors, ",") != "passage-ferry-register,passage-second-view" {
		t.Fatal(chapter)
	}
	get("cowboy-bebop-tv/scenes/ferry-register", 404)
	// A character in another chapter cannot be inserted into this scene.
	if _, e := db.Exec(ctx, sqltest.InsertUnsupportedAppearance, v); e == nil {
		t.Fatal("unsubstantiated scene appearance accepted")
	}
}

func TestEnrichmentKeepsPublishedRevision(t *testing.T) {
	db := testsupport.Database(t)
	ctx := context.Background()
	official := `<div class="storybox"><img alt="#1　Episode"><p class="storytxt">アシモフを追うスパイクとジェット アシモフは恋人のカテリーナと共に 売人のバーテンの店を訪れる</p></div>
 <div class="storybox"><img alt="#2　Episode"><p class="storytxt">一匹の犬を巡ってチャイナタウン アインの登場秘話 スパイク</p></div>`
	var calls, conditional, limited atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		switch r.URL.Path {
		case "/episodes":
			io.WriteString(w, `{"total":26,"data":[{"id":519,"sort":1,"subject_id":253,"name":"アステロイド Asteroid Blues"},{"id":7027,"sort":2,"subject_id":253,"name":"野良犬 Stray Dog Strut"}]}`)
		case "/subjects/253/characters":
			io.WriteString(w, `[{"id":77,"name":"スパイク"},{"id":78,"name":"ジェット"},{"id":170324,"name":"アシモフ"},{"id":170325,"name":"カテリーナ"},{"id":81,"name":"アイン"},{"id":170331,"name":"ハキム"}]`)
		case "/official":
			if limited.Load() == 1 {
				w.Header().Set("Retry-After", "120")
				w.WriteHeader(429)
				return
			}
			w.Header().Set("ETag", `"verified"`)
			if r.Header.Get("If-None-Match") == `"verified"` {
				conditional.Add(1)
				w.WriteHeader(304)
				return
			}
			io.WriteString(w, official)
		case "/chase":
			io.WriteString(w, `<div class="shortSummaryText">A bounty takes Spike and Jet to Mars. Hakim, Spike, and the group of scientists give chase.</div>`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	var work string
	if e := db.QueryRow(ctx, sqltest.CreateBebopWork).Scan(&work); e != nil {
		t.Fatal(e)
	}
	for _, q := range []struct {
		sql string
		arg string
	}{
		{sqltest.AttachBebopSource, work},
		{sqltest.SetBebopURLs, server.URL},
		{sqltest.SetSupplementURL, server.URL},
	} {
		if _, e := db.Exec(ctx, q.sql, q.arg); e != nil {
			t.Fatal(e)
		}
	}
	run := func() error { return domain.Sweep(ctx, db, "bebop-tv-guide", true, false) }
	if e := run(); e != nil {
		t.Fatal(e)
	}
	if e := run(); e != nil || conditional.Load() != 1 {
		t.Fatalf("conditional recheck: %v (%d)", e, conditional.Load())
	}
	limited.Store(1)
	if e := run(); e == nil {
		t.Fatal("rate-limited source published")
	}
	before := calls.Load()
	if e := run(); e != nil || before != calls.Load() {
		t.Fatal("Retry-After ignored", e)
	}
	var revision int
	var actors []string
	if e := db.QueryRow(ctx, sqltest.PublishedCast).Scan(&revision, &actors); e != nil || revision != 1 || strings.Join(actors, ",") != "asimov,katerina" {
		t.Fatalf("published cast changed: %d %v %v", revision, actors, e)
	}
	var hasCover bool
	var portraits, covers int
	if err := db.QueryRow(ctx, sqltest.ArtCompleteness, work).Scan(&hasCover, &portraits, &covers); err != nil || !hasCover || portraits != 2 || covers != 2 {
		t.Fatalf("published reader artwork: cover=%v portraits=%d chapters=%d err=%v", hasCover, portraits, covers, err)
	}
}
