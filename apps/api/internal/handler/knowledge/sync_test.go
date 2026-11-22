package knowledge_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	catalog "anime-agent/api/internal/model/knowledge"
	routing "anime-agent/api/internal/router"
	domain "anime-agent/api/internal/service/knowledge"
	"anime-agent/api/internal/testsupport"
	sqltest "anime-agent/api/internal/testsupport/fixtures"
)

func fixtureAnime(id int, name string) map[string]any {
	return map[string]any{"sources": []string{fmt.Sprintf("https://myanimelist.net/anime/%d", id)}, "title": name, "type": "TV", "episodes": 12, "status": "FINISHED", "animeSeason": map[string]any{"season": "SPRING", "year": 2020}, "picture": "https://example.org/p", "thumbnail": "https://example.org/t", "synonyms": []string{}, "relatedAnime": []string{}, "studios": []string{"Studio"}, "producers": []string{}, "tags": []string{"drama"}}
}
func lines(entries ...map[string]any) []byte {
	var b bytes.Buffer
	e := json.NewEncoder(&b)
	e.Encode(map[string]any{"$schema": "https://example.org/schema", "lastUpdate": "2020-01-01",
		"repository": "https://github.com/cedya77/anime-offline-database", "scoreRange": map[string]int{"minInclusive": 1, "maxInclusive": 10},
		"license": map[string]string{"name": "Open Data Commons Open Database License (ODbL) v1.0 + Database Contents License (DbCL) v1.0", "url": "https://example.org/license"}})
	for _, x := range entries {
		e.Encode(x)
	}
	return b.Bytes()
}
func pack(t *testing.T, dist any) []byte {
	t.Helper()
	body, err := json.Marshal(dist)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err = tw.WriteHeader(&tar.Header{Name: "package/dist/data.json", Size: int64(len(body)), Mode: 0644}); err != nil {
		t.Fatal(err)
	}
	if _, err = tw.Write(body); err != nil {
		t.Fatal(err)
	}
	if err = tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err = gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// Only the newly created per-test schema is modified. The DSN must be a PostgreSQL URL.
func TestPostgresSnapshotLifecycle(t *testing.T) {
	t.Setenv("KNOWLEDGE_ARCHIVE_DIR", t.TempDir())
	ctx := context.Background()
	db := testsupport.Database(t)
	var err error
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, e := db.Exec(ctx, sql, args...); e != nil {
			t.Fatal(e)
		}
	}
	one := func(sql string, args ...any) string {
		t.Helper()
		var v string
		if e := db.QueryRow(ctx, sql, args...).Scan(&v); e != nil {
			t.Fatal(e)
		}
		return v
	}
	npmVersion := "1.0"
	version := 1
	body := lines(fixtureAnime(1, "Shared"), fixtureAnime(2, "Second"))
	badDigest := false
	truncated := false
	downloads := 0
	bg := pack(t, map[string]any{"siteMeta": map[string]any{"mal": map[string]any{"title": "provider", "type": "info", "urlTemplate": "https://myanimelist.net/anime/{{id}}"}}, "items": []any{
		map[string]any{"title": "共有", "lang": "ja", "titleTranslate": map[string][]string{"zh-Hans": {"共享"}}, "type": "tv", "begin": "2020-01-01T00:00:00Z", "end": "", "sites": []any{map[string]string{"site": "mal", "id": "1"}}},
		map[string]any{"title": "Second", "lang": "en", "titleTranslate": map[string]any{}, "type": "tv", "begin": "2020-03-01T00:00:00Z", "end": "", "sites": []any{}},
	}})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/release":
			h := sha256.Sum256(body)
			digest := hex.EncodeToString(h[:])
			if badDigest {
				digest = strings.Repeat("0", 64)
			}
			json.NewEncoder(w).Encode(map[string]any{"tag_name": "latest", "assets": []any{map[string]any{"name": "anime-offline-database.jsonl", "id": version, "size": len(body), "digest": "sha256:" + digest}}})
		case "/aod":
			downloads++
			if truncated {
				w.Header().Set("Content-Length", fmt.Sprint(len(body)))
				w.Write(body[:len(body)/2])
				return
			}
			w.Write(body)
		case "/npm":
			h := sha512.Sum512(bg)
			json.NewEncoder(w).Encode(map[string]any{"name": "bangumi-data", "main": "dist/data.json", "version": npmVersion, "dist": map[string]string{"tarball": "http://" + r.Host + "/dist.tgz", "integrity": "sha512-" + base64.StdEncoding.EncodeToString(h[:])}})
		case "/dist.tgz":
			w.Write(bg)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	const A = "anime-offline-database"
	const B = "bangumi-data"
	exec(sqltest.SetSourceURLs, srv.URL+"/release", srv.URL+"/aod", A)
	exec(sqltest.SetSourceURLs, srv.URL+"/npm", srv.URL, B)
	run := func(name string, success bool) {
		t.Helper()
		if e := domain.Sweep(ctx, db, name, true, false); (e == nil) != success {
			t.Fatalf("run %s expected success=%t: %v", name, success, e)
		}
	}
	count := func(sql, want string, args ...any) {
		t.Helper()
		if got := one(sql, args...); got != want {
			t.Fatalf("%s: want %s, got %s", sql, want, got)
		}
	}
	run(A, true)
	work := one(sqltest.FindSharedWork)
	run(B, true)
	count(sqltest.CountActiveWorks, "3")
	count(sqltest.CountWorkRecords, "2", work)
	count(sqltest.CountTitleMatches, "2")
	run(A, true)
	if downloads != 1 {
		t.Fatal("unchanged release downloaded")
	}
	version++
	body = lines(fixtureAnime(1, "Changed"), fixtureAnime(2, "Second"))
	badDigest = true
	run(A, false)
	badDigest = false
	truncated = true
	run(A, false)
	truncated = false
	count(sqltest.WorkTitle, "Shared", work)
	many := make([]map[string]any, 401)
	for i := range many {
		many[i] = fixtureAnime(100+i, "stage")
	}
	body = append(lines(many...), []byte("{bad json\n")...)
	run(A, false)
	count(sqltest.CountStaging, "0")
	count(sqltest.CountWorks, "3")
	body = lines(fixtureAnime(1, "Changed"), fixtureAnime(2, "Second"))
	exec(sqltest.RejectFixtureTitle)
	run(A, false)
	count(sqltest.WorkTitle, "Shared", work)
	exec(sqltest.AllowFixtureTitle)
	run(A, true)
	count(sqltest.WorkTitle, "Changed", work)
	count(sqltest.LatestChangedCounts, "1/1")
	version++
	body = lines(fixtureAnime(2, "Second"))
	run(A, false) // large-drop guard
	exec(sqltest.SetRetention, A)
	run(A, true)
	count(sqltest.ActiveWorkTitle, "共有", work)
	count(sqltest.LatestDeletionCount, "1")
	count(sqltest.CountRunningSyncs, "0")
	// A worker that died after staging must leave neither a published batch nor stale rows.
	var orphan string
	if e := db.QueryRow(ctx, sqltest.CreateOrphanRun, A).Scan(&orphan); e != nil {
		t.Fatal(e)
	}
	exec(sqltest.CreateOrphanRecord, orphan)
	run(A, true)
	count(sqltest.SyncStatus, "failed", orphan)
	count(sqltest.CountStaging, "0")
	// Same-provider evidence that contradicts another exact ID is review-only.
	version++
	conflict := fixtureAnime(1, "Changed")
	conflict["sources"] = []string{"https://myanimelist.net/anime/1", "https://anidb.net/anime/999"}
	body = lines(conflict, fixtureAnime(2, "Second"))
	run(A, true)
	// The existing Bangumi snapshot now gains conflicting AniDB evidence.
	bg = pack(t, map[string]any{"siteMeta": map[string]any{"mal": map[string]string{"title": "provider", "type": "info", "urlTemplate": "https://myanimelist.net/anime/{{id}}"}, "anidb": map[string]string{"title": "provider", "type": "info", "urlTemplate": "https://anidb.net/anime/{{id}}"}}, "items": []any{
		map[string]any{"title": "共有", "lang": "ja", "titleTranslate": map[string]any{}, "type": "tv", "begin": "2020-01-01T00:00:00Z", "end": "", "sites": []map[string]string{{"site": "mal", "id": "1"}, {"site": "anidb", "id": "888"}}},
	}})
	exec(sqltest.SetRetention, B)
	npmVersion = "1.1"
	run(B, true)
	count(sqltest.CountDistinctProviderWorks, "2")
	count(sqltest.CountProviderConflicts, "2")
	lock, err := db.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = lock.Exec(ctx, sqltest.LockSource, A); err != nil {
		t.Fatal(err)
	}
	run(A, true)
	count(sqltest.LatestSkipReason, "already_running")
	lock.Exec(ctx, sqltest.UnlockSession)
	lock.Release()
	before := one(sqltest.CountSyncRuns)
	if err = domain.Sweep(ctx, db, A, false, false); err != nil {
		t.Fatal(err)
	}
	count(sqltest.CountSyncRuns, before)
	exec(sqltest.AgeLastCheck, A)
	if err = domain.Sweep(ctx, db, A, false, false); err != nil {
		t.Fatal(err)
	}
	count(sqltest.LatestSkipReason, "version_unchanged")
	report, e := domain.Quality(ctx, db, "all", "")
	if e != nil {
		t.Fatal(e)
	}
	t.Log(string(report))
	var candidate, record int64
	if err = db.QueryRow(ctx, sqltest.FirstMatchRecord).Scan(&candidate, &record); err != nil {
		t.Fatal(err)
	}
	decide := func(d catalog.Decision) {
		t.Helper()
		d.Reason = "fixture review"
		if e := domain.Decide(ctx, db, d, "fixture", "test"); e != nil {
			t.Fatal(e)
		}
	}
	decide(catalog.Decision{Action: "reject", Candidate: candidate})
	decide(catalog.Decision{Action: "rematch", Record: record})
	if err = db.QueryRow(ctx, sqltest.MatchForRecord, record).Scan(&candidate); err != nil {
		t.Fatal(err)
	}
	decide(catalog.Decision{Action: "approve", Candidate: candidate})
	decide(catalog.Decision{Action: "split", Record: record})
	work = one(sqltest.WorkBySourceKey)
	decide(catalog.Decision{Action: "override", Work: work, Field: "canonical_title", Value: json.RawMessage(`"Reviewed"`)})
	first := one(sqltest.FirstSourceVersion)
	latest := one(sqltest.SourceVersion)
	delta, e := domain.DiffSnapshots(ctx, db, A, first, latest, "mal:1")
	if e != nil || string(delta["title"][1]) != `"Changed"` {
		t.Fatal(delta, e)
	}
	if err = domain.Reproject(ctx, db, A); err != nil {
		t.Fatal(err)
	}
	engine := gin.New()
	routing.Register(engine, db)
	token := strings.Repeat("a", 32)
	t.Setenv("KNOWLEDGE_ADMIN_TOKEN", token)
	t.Setenv("KNOWLEDGE_ADMIN_OPERATOR", "tester")
	route := func(path string, auth bool, want int) {
		t.Helper()
		req := httptest.NewRequest("GET", "/api/v1/admin/knowledge/"+path, nil)
		if auth {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)
		if w.Code != want {
			t.Fatal(path, w.Code, w.Body.String())
		}
	}
	route("matches", false, 401)
	route("matches", true, 200)
	route("works/"+work+"/issues", true, 200)
	route("works/"+work+"/history", true, 200)
	match := one(sqltest.FirstMatchID)
	route("matches/"+match, true, 200)
	aiCalls := 0
	var cited int64
	if err = db.QueryRow(ctx, sqltest.EvidenceCandidate, match).Scan(&cited); err != nil {
		t.Fatal(err)
	}
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		aiCalls++
		a := gin.H{"verdict": "uncertain", "reason": "需人审核", "evidence": []gin.H{{"field": "title", "candidate_record_id": cited, "observation": "比对两个标题"}}}
		content, _ := json.Marshal(a)
		json.NewEncoder(w).Encode(gin.H{"usage": gin.H{"prompt_tokens": 5, "completion_tokens": 2}, "choices": []gin.H{{"message": gin.H{"content": string(content)}}}})
	}))
	defer provider.Close()
	t.Setenv("MATCH_AI_BASE_URL", provider.URL)
	t.Setenv("MATCH_AI_MODEL", "fixture")
	assess := func(expected int) {
		t.Helper()
		r := httptest.NewRequest("POST", "/api/v1/admin/knowledge/matches/"+match+"/assessments", nil)
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, r)
		if w.Code != expected {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	assess(200)
	assess(200)
	route("matches/"+match+"/assessments", true, 200)
	if aiCalls != 1 {
		t.Fatal("expected cached assessment")
	}
	count(sqltest.CountAssessments, "1")
	count(sqltest.CountMeteredMatches, "1")
	cited = -1
	t.Setenv("MATCH_AI_MODEL", "different-fixture")
	assess(502)
	count(sqltest.CountAssessments, "1")
	count(sqltest.CountMeteredMatches, "2")

	if err = domain.Rebuild(ctx, db, A, first); err != nil {
		t.Fatal(err)
	}
	count(sqltest.WorkTitle, "Reviewed", work)
	decide(catalog.Decision{Action: "undo", Work: work, Field: "canonical_title"})
	count(sqltest.WorkTitle, "Shared", work)
	count(sqltest.CountDecisions, "6")
	for _, statement := range []string{sqltest.SeedOpsSyncRuns, sqltest.SeedOpsWorlds} {
		if _, err = db.Exec(ctx, statement); err != nil {
			t.Fatalf("seed published worlds for scheduled completeness: %v", err)
		}
	}
	if err = domain.QualityIfDue(ctx, db); err != nil {
		t.Fatal(err)
	}
	if err = domain.QualityIfDue(ctx, db); err != nil {
		t.Fatal(err)
	}
	count(sqltest.CountQualityReports, "1")
	var completenessRows int
	if err = db.QueryRow(ctx, sqltest.CompletenessResultCount).Scan(&completenessRows); err != nil || completenessRows == 0 {
		t.Fatalf("scheduled quality scan did not store completeness results: rows=%d err=%v", completenessRows, err)
	}
	exec(sqltest.AgeQualityRun)
	if err = domain.QualityIfDue(ctx, db); err != nil {
		t.Fatal(err)
	}
	count(sqltest.CountSuccessfulScans, "2")

	// A quality check must only write governance rows. Resolving a field then
	// scanning the same work a second time closes the original issue.
	decide(catalog.Decision{Action: "override", Work: work, Field: "canonical_title", Value: json.RawMessage(`"  Shared  "`)})
	if _, e = domain.Quality(ctx, db, "all", work); e != nil {
		t.Fatal(e)
	}
	count(sqltest.WorkTitle, "  Shared  ", work)
	count(sqltest.TitleIssueStatus, "open", work)
	decide(catalog.Decision{Action: "undo", Work: work, Field: "canonical_title"})
	if _, e = domain.Quality(ctx, db, A, work); e != nil {
		t.Fatal(e)
	}
	count(sqltest.TitleIssueStatus, "resolved", work)
	key := one(sqltest.SnapshotByVersion, first)
	path, e := domain.ArchivePath(key)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(path, []byte("corrupt fixture"), 0600); e != nil {
		t.Fatal(e)
	}
	if e = domain.Rebuild(ctx, db, A, first); e == nil {
		t.Fatal("corrupt archive accepted")
	}
	count(sqltest.WorkTitle, "Shared", work)

}
