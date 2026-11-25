package knowledge_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	routing "anime-agent/api/internal/router"
	domain "anime-agent/api/internal/service/knowledge"
	"anime-agent/api/internal/testsupport"
	fixture "anime-agent/api/internal/testsupport/fixtures"
)

func TestP1SourceReviewLifecycle(t *testing.T) {
	db := testsupport.Database(t)
	ctx := context.Background()
	t.Setenv("KNOWLEDGE_ARCHIVE_DIR", t.TempDir())
	q := fixture.CurationP1
	exec := func(statement string, args ...any) {
		t.Helper()
		if _, err := db.Exec(ctx, statement, args...); err != nil {
			t.Fatal(err)
		}
	}
	for _, statement := range []string{fixture.SeedOpsSyncRuns, fixture.SeedOpsWorlds, fixture.SeedCompletenessExamples,
		fixture.SeedCompletenessCatalogOnly, fixture.SeedCompletenessP0Contracts, q["BindCatalogProviderToWorld"], q["AllowSourceWithdrawal"]} {
		exec(statement)
	}

	const source = "anime-offline-database"
	version := 900
	a := fixtureAnime(987651, "P1 World")
	a["episodes"] = 26
	b := fixtureAnime(987652, "P1 Catalog")
	body := lines(a, b)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/release" {
			json.NewEncoder(w).Encode(map[string]any{"tag_name": "p1", "assets": []any{map[string]any{
				"name": "anime-offline-database.jsonl", "id": version, "size": len(body), "digest": fmt.Sprintf("sha256:%x", sha256.Sum256(body)),
			}}})
			return
		}
		w.Write(body)
	}))
	defer upstream.Close()
	exec(fixture.SetSourceURLs, upstream.URL+"/release", upstream.URL+"/aod", source)
	sweep := func(refresh bool) {
		t.Helper()
		if err := domain.Sweep(ctx, db, source, true, refresh); err != nil {
			t.Fatal(err)
		}
	}

	router := gin.New()
	routing.Register(router, db)
	paths := []string{"worlds/cowboy-bebop-tv", "worlds/cowboy-bebop-tv/chapters/phase2-extension-3", "worlds/cowboy-bebop-tv/scenes/phase2-extension-scene-3"}
	public := func() [][]byte {
		t.Helper()
		bodies := make([][]byte, len(paths))
		for i, path := range paths {
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/knowledge/"+path, nil))
			if response.Code != 200 {
				t.Fatalf("public %s HTTP=%d %s", path, response.Code, response.Body)
			}
			bodies[i] = response.Body.Bytes()
		}
		return bodies
	}
	baseline := public()
	unchangedPublic := func() {
		t.Helper()
		for i, body := range public() {
			if !bytes.Equal(body, baseline[i]) {
				t.Errorf("public payload changed for %s", paths[i])
			}
		}
	}
	type delta struct{ Before, After any }
	type sourceMark struct {
		ID                string
		LastRunID         string `json:"last_run_id"`
		PreviousVersionID *int64 `json:"previous_version_id"`
		SourceVersionID   int64  `json:"source_version_id"`
		Changes           struct {
			Kind            string
			Fields, Catalog map[string]delta
			Raw, Norm       delta
		}
	}
	mark := func(kind string) sourceMark {
		t.Helper()
		var m sourceMark
		var changes []byte
		if err := db.QueryRow(ctx, q["OpenCatalogReview"], "cowboy-bebop-tv").Scan(
			&m.ID, &m.LastRunID, &m.PreviousVersionID, &m.SourceVersionID, &changes,
		); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(changes, &m.Changes); err != nil || m.Changes.Kind != kind {
			t.Fatalf("want %s review, got %s (err=%v)", kind, changes, err)
		}
		return m
	}
	count := func(query string, expected int, args ...any) {
		t.Helper()
		var got int
		if err := db.QueryRow(ctx, q[query], args...).Scan(&got); err != nil || got != expected {
			t.Fatalf("%s got=%d want=%d err=%v", query, got, expected, err)
		}
	}

	sweep(false)
	initial := mark("added")
	exec(q["MarkFixtureReviewResolved"], initial.ID)

	version++
	a["type"] = "MOVIE"
	a["episodes"] = 1
	b["episodes"] = 19
	body = lines(a, b)
	sweep(false)
	changed := mark("updated")
	if changed.ID == initial.ID || changed.PreviousVersionID == nil || *changed.PreviousVersionID != initial.SourceVersionID ||
		changed.Changes.Catalog["type"] != (delta{"TV", "MOVIE"}) ||
		changed.Changes.Fields["episodes"] != (delta{float64(26), float64(1)}) ||
		changed.Changes.Raw.Before == changed.Changes.Raw.After || changed.Changes.Norm.Before == changed.Changes.Norm.After {
		t.Fatalf("source diff missing typed changes or hashes: %+v", changed)
	}
	publicAndCatalog := func() {
		t.Helper()
		unchangedPublic()
		for _, entry := range []struct {
			key, kind string
			episodes  int
		}{{"mal:987651", "TV", 1}, {"mal:987652", "TV", 19}} {
			var active bool
			var kind string
			var episodes int
			if err := db.QueryRow(ctx, q["CatalogWork"], entry.key).Scan(&active, &kind, &episodes); err != nil || !active || kind != entry.kind || episodes != entry.episodes {
				t.Fatalf("catalog %s active=%v type=%s episodes=%d err=%v", entry.key, active, kind, episodes, err)
			}
		}
	}
	publicAndCatalog()
	sweep(true)
	repeated := mark("updated")
	if repeated.ID != changed.ID || repeated.LastRunID != changed.LastRunID {
		t.Fatal("refresh duplicated or replaced unchanged review")
	}
	if err := domain.QualityIfDue(ctx, db); err != nil {
		t.Fatal(err)
	}
	count("SuccessfulQualityRuns", 1)
	count("OpenReviews", 1)
	t.Log("type proposal TV to MOVIE, world/chapters/scenes unchanged, unchanged refresh and quality scan kept one open review")

	version++
	a["picture"] = "https://example.org/new-image"
	body = lines(a, b)
	sweep(false)
	rawOnly := mark("updated")
	if rawOnly.ID != changed.ID || rawOnly.LastRunID == changed.LastRunID || len(rawOnly.Changes.Fields) != 0 ||
		rawOnly.Changes.Raw.Before == rawOnly.Changes.Raw.After || rawOnly.Changes.Norm.Before != rawOnly.Changes.Norm.After {
		t.Fatalf("raw-only change not distinguished: %+v", rawOnly)
	}

	version++
	body = lines(b)
	sweep(false)
	withdrawn := mark("withdrawn")
	if withdrawn.Changes.Catalog["active"] != (delta{true, false}) || withdrawn.Changes.Raw.After != nil {
		t.Fatalf("missing proposed withdrawal: %+v", withdrawn)
	}
	publicAndCatalog()
	t.Log("withdrawing the last contribution proposed deactivation and preserved public payloads")
}
