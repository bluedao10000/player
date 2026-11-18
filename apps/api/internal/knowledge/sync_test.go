package knowledge

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
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
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
func TestParseAOD(t *testing.T) {
	n := 0
	_, err := parseAOD(bytes.NewReader(lines(fixtureAnime(10, "First"))), func(key string, raw json.RawMessage, w normalized) error {
		n++
		if key != "mal:10" || w.Title != "First" || w.Year != 2020 {
			t.Fatal(key, w)
		}
		return nil
	})
	if err != nil || n != 1 {
		t.Fatal(n, err)
	}
	x := fixtureAnime(10, "Bad")
	delete(x, "animeSeason")
	_, err = parseAOD(bytes.NewReader(lines(x)), func(string, json.RawMessage, normalized) error { t.Fatal("invalid row emitted"); return nil })
	if err == nil {
		t.Fatal("invalid row accepted")
	}
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
	dsn := os.Getenv("KNOWLEDGE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set KNOWLEDGE_TEST_DATABASE_URL for PostgreSQL integration")
	}
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(ctx)
	ns := fmt.Sprintf("kb_test_%d", time.Now().UnixNano())
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+ns); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(ctx, "DROP SCHEMA "+ns+" CASCADE")
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	q.Set("search_path", ns)
	u.RawQuery = q.Encode()
	t.Setenv("DATABASE_URL", u.String())
	db, err := Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
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
	exec("UPDATE knowledge_sources SET check_url=$1,data_url=$2 WHERE name=$3", srv.URL+"/release", srv.URL+"/aod", A)
	exec("UPDATE knowledge_sources SET check_url=$1,data_url=$2 WHERE name=$3", srv.URL+"/npm", srv.URL, B)
	run := func(name string, success bool) {
		t.Helper()
		if e := Sweep(ctx, db, name, true, false); (e == nil) != success {
			t.Fatalf("run %s expected success=%t: %v", name, success, e)
		}
	}
	count := func(sql, want string) {
		t.Helper()
		if got := one(sql); got != want {
			t.Fatalf("%s: want %s, got %s", sql, want, got)
		}
	}
	run(A, true)
	work := one("SELECT id::text FROM works WHERE canonical_title='Shared'")
	run(B, true)
	count("SELECT count(*)::text FROM works WHERE active", "3")
	count("SELECT count(*)::text FROM knowledge_source_records WHERE work_id='"+work+"'", "2")
	count("SELECT count(*)::text FROM knowledge_match_candidates WHERE reason='title_year'", "2")
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
	count("SELECT canonical_title FROM works WHERE id='"+work+"'", "Shared")
	many := make([]map[string]any, 401)
	for i := range many {
		many[i] = fixtureAnime(100+i, "stage")
	}
	body = append(lines(many...), []byte("{bad json\n")...)
	run(A, false)
	count("SELECT count(*)::text FROM staging_source_records", "0")
	count("SELECT count(*)::text FROM works", "3")
	body = lines(fixtureAnime(1, "Changed"), fixtureAnime(2, "Second"))
	exec("ALTER TABLE works ADD CONSTRAINT fixture_reject CHECK(canonical_title<>'Changed')")
	run(A, false)
	count("SELECT canonical_title FROM works WHERE id='"+work+"'", "Shared")
	exec("ALTER TABLE works DROP CONSTRAINT fixture_reject")
	run(A, true)
	count("SELECT canonical_title FROM works WHERE id='"+work+"'", "Changed")
	count("SELECT records_updated||'/'||records_unchanged FROM knowledge_sync_runs ORDER BY started_at DESC LIMIT 1", "1/1")
	version++
	body = lines(fixtureAnime(2, "Second"))
	run(A, false) // large-drop guard
	exec("UPDATE knowledge_sources SET min_retained_ratio=0.5 WHERE name=$1", A)
	run(A, true)
	count("SELECT canonical_title FROM works WHERE id='"+work+"' AND active", "共有")
	count("SELECT records_deleted::text FROM knowledge_sync_runs ORDER BY started_at DESC LIMIT 1", "1")
	count("SELECT count(*)::text FROM knowledge_sync_runs WHERE status='running'", "0")
	// A worker that died after staging must leave neither a published batch nor stale rows.
	var orphan string
	if e := db.QueryRow(ctx, "INSERT INTO knowledge_sync_runs(source_id,status) SELECT id,'running' FROM knowledge_sources WHERE name=$1 RETURNING id::text", A).Scan(&orphan); e != nil {
		t.Fatal(e)
	}
	exec("INSERT INTO staging_source_records VALUES($1,'orphan','{}','{}')", orphan)
	run(A, true)
	count("SELECT status FROM knowledge_sync_runs WHERE id='"+orphan+"'", "failed")
	count("SELECT count(*)::text FROM staging_source_records", "0")
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
	exec("UPDATE knowledge_sources SET min_retained_ratio=0.5 WHERE name=$1", B)
	npmVersion = "1.1"
	run(B, true)
	count("SELECT count(DISTINCT work_id)::text FROM knowledge_source_records WHERE active AND norm->'refs' @> '[{\"provider\":\"mal\",\"id\":\"1\"}]'", "2")
	count("SELECT count(*)::text FROM knowledge_match_candidates WHERE reason='provider_conflict'", "2")
	lock, err := db.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = lock.Exec(ctx, "SELECT pg_advisory_lock(82045,id) FROM knowledge_sources WHERE name=$1", A); err != nil {
		t.Fatal(err)
	}
	run(A, true)
	count("SELECT skip_reason FROM knowledge_sync_runs ORDER BY started_at DESC LIMIT 1", "already_running")
	lock.Exec(ctx, "SELECT pg_advisory_unlock_all()")
	lock.Release()
	before := one("SELECT count(*)::text FROM knowledge_sync_runs")
	if err = Sweep(ctx, db, A, false, false); err != nil {
		t.Fatal(err)
	}
	count("SELECT count(*)::text FROM knowledge_sync_runs", before)
	exec("UPDATE knowledge_sources SET last_checked_at=now()-interval '7 hours' WHERE name=$1", A)
	if err = Sweep(ctx, db, A, false, false); err != nil {
		t.Fatal(err)
	}
	count("SELECT skip_reason FROM knowledge_sync_runs ORDER BY started_at DESC LIMIT 1", "version_unchanged")
}
