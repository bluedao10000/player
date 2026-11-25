package knowledge_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	catalog "anime-agent/api/internal/model/knowledge"
	routing "anime-agent/api/internal/router"
	"anime-agent/api/internal/testsupport"
	fixture "anime-agent/api/internal/testsupport/fixtures"
)

func TestP1CurationProtectsPublishedWorld(t *testing.T) {
	db := testsupport.Database(t)
	ctx := context.Background()
	q := fixture.CurationP1
	exec := func(statement string, args ...any) {
		t.Helper()
		if _, err := db.Exec(ctx, statement, args...); err != nil {
			t.Fatal(err)
		}
	}
	readText := func(statement string, args ...any) string {
		t.Helper()
		var value string
		if err := db.QueryRow(ctx, statement, args...).Scan(&value); err != nil {
			t.Fatal(err)
		}
		return value
	}
	record := func(key string) (int64, string) {
		t.Helper()
		var id int64
		var work string
		if err := db.QueryRow(ctx, q["SelectRecord"], key).Scan(&id, &work); err != nil {
			t.Fatal(err)
		}
		return id, work
	}
	match := func(source int64, target string) int64 {
		t.Helper()
		var id int64
		if err := db.QueryRow(ctx, q["Candidate"], source, target).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}

	for _, statement := range []string{fixture.SeedOpsSyncRuns, fixture.SeedOpsWorlds, fixture.SeedCompletenessExamples,
		fixture.SeedCompletenessCatalogOnly, fixture.SeedCompletenessP0Contracts, q["Seed"]} {
		exec(statement)
	}
	world := readText(q["SelectWorld"])
	worldRecord, _ := record("p1-world")
	catalogA, workA := record("p1-catalog-a")
	catalogB, workB := record("p1-catalog-b")
	fromWorld := match(worldRecord, workA)
	toWorld := match(catalogA, world)
	exec(q["RestoreHistory"], world, workA, catalogA)

	var pic bytes.Buffer
	if err := jpeg.Encode(&pic, image.NewRGBA(image.Rect(0, 0, 2, 2)), nil); err != nil {
		t.Fatal(err)
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(pic.Bytes()))
	archive := t.TempDir()
	t.Setenv("KNOWLEDGE_ARCHIVE_DIR", archive)
	if err := os.WriteFile(filepath.Join(archive, digest), pic.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	exec(q["SetPublishedImage"], "/art/episode-scenes/"+digest+".jpg")

	t.Setenv("KNOWLEDGE_ADMIN_TOKEN", "12345678901234567890123456789012")
	t.Setenv("KNOWLEDGE_ADMIN_OPERATOR", "fixture")
	router := gin.New()
	routing.Register(router, db)
	publicPaths := []string{
		"worlds/cowboy-bebop-tv",
		"worlds/cowboy-bebop-tv/chapters/cb-chapter-1",
		"worlds/cowboy-bebop-tv/chapters/phase2-extension-3",
		"worlds/cowboy-bebop-tv/scenes/cb-scene-1",
		"worlds/cowboy-bebop-tv/scenes/phase2-extension-scene-3",
		"works/" + world,
	}
	readPublic := func(t *testing.T) map[string][]byte {
		t.Helper()
		bodies := make(map[string][]byte)
		for _, path := range publicPaths {
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/knowledge/"+path, nil))
			if response.Code != http.StatusOK {
				t.Fatalf("%s HTTP=%d body=%s", path, response.Code, response.Body)
			}
			bodies[path] = response.Body.Bytes()
		}
		return bodies
	}
	decide := func(t *testing.T, decision catalog.Decision) *httptest.ResponseRecorder {
		t.Helper()
		decision.Reason = "P1 identity fixture"
		body, err := json.Marshal(decision)
		if err != nil {
			t.Fatal(err)
		}
		request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/knowledge/actions", bytes.NewReader(body))
		request.Header.Set("Authorization", "Bearer 12345678901234567890123456789012")
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		return response
	}

	baseline := readPublic(t)
	if !bytes.Contains(baseline[publicPaths[4]], []byte(digest)) ||
		!bytes.Contains(baseline[publicPaths[0]], []byte(`"format": "series"`)) ||
		!bytes.Contains(baseline[publicPaths[3]], []byte("#passage-cb-scene-1")) {
		t.Fatal("fixture lacks the public format, anchor or selected JPEG")
	}
	assertRejected := func(t *testing.T, decision catalog.Decision) {
		t.Helper()
		before := readText(q["State"])
		publicBefore := readPublic(t)
		response := decide(t, decision)
		var conflict struct{ Code, Error string }
		if response.Code != http.StatusConflict || json.Unmarshal(response.Body.Bytes(), &conflict) != nil ||
			conflict.Code != "work_identity_protected" || !strings.Contains(conflict.Error, "作品世界") {
			t.Fatalf("expected identity protection, got HTTP %d %s", response.Code, response.Body)
		}
		if after := readText(q["State"]); before != after {
			t.Fatal("rejected decision changed database state")
		}
		for path, body := range readPublic(t) {
			if !bytes.Equal(body, publicBefore[path]) {
				t.Errorf("public response changed at %s", path)
			}
		}
		t.Logf("HTTP %d, database and %d public responses identical", response.Code, len(publicBefore))
	}

	for _, test := range []struct {
		name   string
		action catalog.Decision
	}{
		{"merge_world_from", catalog.Decision{Action: "merge", Candidate: fromWorld}},
		{"merge_world_to", catalog.Decision{Action: "merge", Candidate: toWorld}},
		{"split_last_record", catalog.Decision{Action: "split", Record: worldRecord}},
		{"split_restore_world", catalog.Decision{Action: "split", Record: catalogA, Work: world}},
		{"approve_world_from", catalog.Decision{Action: "approve", Candidate: fromWorld}},
		{"approve_world_to", catalog.Decision{Action: "approve", Candidate: toWorld}},
		{"rematch_world", catalog.Decision{Action: "rematch", Record: worldRecord}},
	} {
		t.Run(test.name, func(t *testing.T) { assertRejected(t, test.action) })
	}
	t.Run("split_with_other_record", func(t *testing.T) {
		exec(q["AddRetainedRecord"])
		assertRejected(t, catalog.Decision{Action: "split", Record: worldRecord})
	})

	t.Run("catalog_merge_and_split", func(t *testing.T) {
		candidate := match(catalogA, workB)
		response := decide(t, catalog.Decision{Action: "merge", Candidate: candidate})
		if _, owner := record("p1-catalog-a"); response.Code != http.StatusNoContent || owner != workB {
			t.Fatalf("catalog merge HTTP=%d body=%s owner=%s", response.Code, response.Body, owner)
		}
		response = decide(t, catalog.Decision{Action: "split", Record: catalogA})
		if _, owner := record("p1-catalog-a"); response.Code != http.StatusNoContent || owner == workB || owner == workA {
			t.Fatalf("catalog split HTTP=%d body=%s owner=%s", response.Code, response.Body, owner)
		}
		t.Log("catalog merge and split both returned 204 and changed ownership")
	})
	t.Run("historical_release_without_world", func(t *testing.T) {
		exec(q["ArchiveCatalogWork"], workB)
		assertRejected(t, catalog.Decision{Action: "split", Record: catalogB})
	})
	t.Run("inactive_historical_release_as_target", func(t *testing.T) {
		exec(q["DeactivateCatalogWork"], workB)
		assertRejected(t, catalog.Decision{Action: "merge", Candidate: match(catalogA, workB)})
	})
}
