package knowledge_test

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	routing "anime-agent/api/internal/router"
	domain "anime-agent/api/internal/service/knowledge"
	"anime-agent/api/internal/testsupport"
	sqltest "anime-agent/api/internal/testsupport/fixtures"
)

type completenessResponse struct {
	Available   bool                    `json:"available"`
	WorkID      string                  `json:"work_id"`
	WorldSlug   string                  `json:"world_slug"`
	VersionID   int64                   `json:"version_id"`
	RuleVersion int                     `json:"rule_version"`
	Dimensions  []completenessDimension `json:"dimensions"`
}

type completenessDimension struct {
	Key    string                      `json:"key"`
	Status string                      `json:"status"`
	Items  map[string]completenessItem `json:"items"`
}

type completenessItem struct {
	DataState  string  `json:"data_state"`
	Completed  *int    `json:"completed_required_items"`
	Applicable *int    `json:"applicable_required_items"`
	Status     *string `json:"status"`
	Details    []struct {
		ItemKey string `json:"item_key"`
		Status  string `json:"status"`
		Reason  string `json:"reason"`
	} `json:"details"`
}

func TestOpsKnowledgeCompletenessExamplesAndFilters(t *testing.T) {
	db := testsupport.Database(t)
	ctx := context.Background()
	for _, statement := range []string{sqltest.SeedOpsSyncRuns, sqltest.SeedOpsWorlds, sqltest.SeedCompletenessExamples} {
		if _, err := db.Exec(ctx, statement); err != nil {
			t.Fatalf("seed completeness examples: %v", err)
		}
	}
	if err := domain.RecalculateCompleteness(ctx, db); err != nil {
		t.Fatalf("calculate completeness: %v", err)
	}
	var rules int
	if err := db.QueryRow(ctx, sqltest.CompletenessRuleCount).Scan(&rules); err != nil || rules != 17 {
		t.Fatalf("seed rules=%d err=%v, want 17", rules, err)
	}
	var firstCount int
	if err := db.QueryRow(ctx, sqltest.CompletenessResultCount).Scan(&firstCount); err != nil {
		t.Fatal(err)
	}
	if err := domain.RecalculateCompleteness(ctx, db); err != nil {
		t.Fatalf("repeat calculation: %v", err)
	}
	var secondCount int
	if err := db.QueryRow(ctx, sqltest.CompletenessResultCount).Scan(&secondCount); err != nil || secondCount != firstCount {
		t.Fatalf("recalculation changed result row count %d -> %d (err=%v)", firstCount, secondCount, err)
	}

	router := completenessRouter(t, db)
	cowboy := getCompleteness(t, router, "/api/v1/admin/knowledge/ops/knowledge/completeness?world=cowboy-bebop-tv")
	assertRatio(t, cowboy, "basic_profile", "core_fields", 7, 7, "completed")
	assertRatio(t, cowboy, "basic_profile", "episode_total", 1, 1, "completed")
	assertRatio(t, cowboy, "people", "introductions", 6, 8, "partial")
	assertRatio(t, cowboy, "plot", "storylines", 9, 9, "completed")
	assertRatio(t, cowboy, "plot", "chapters", 10, 10, "completed")
	assertRatio(t, cowboy, "plot", "episodes", 10, 26, "partial")
	assertRatio(t, cowboy, "scenes", "scenes", 2, 10, "partial")
	assertRatio(t, cowboy, "world", "locations", 10, 10, "completed")
	assertRatio(t, cowboy, "images", "main_visuals", 1, 1, "completed")
	assertRatio(t, cowboy, "images", "portraits", 8, 8, "completed")
	assertRatio(t, cowboy, "images", "chapter_images", 10, 10, "completed")
	if got := countDetails(cowboy, "scenes", "scenes", "missing"); got != 8 {
		t.Fatalf("cowboy missing scene-text details=%d, want 8", got)
	}
	assertNoData(t, cowboy, "people", "relationships")
	assertNoData(t, cowboy, "world", "organizations")
	assertNoData(t, cowboy, "world", "typed_settings")
	assertNoData(t, cowboy, "timeline", "events")

	tidepost := getCompleteness(t, router, "/api/v1/admin/knowledge/ops/knowledge/completeness?world=tidepost")
	assertRatio(t, tidepost, "basic_profile", "core_fields", 7, 7, "completed")
	assertRatio(t, tidepost, "people", "introductions", 0, 3, "missing")
	assertRatio(t, tidepost, "plot", "storylines", 1, 1, "completed")
	assertRatio(t, tidepost, "plot", "chapters", 4, 4, "completed")
	assertStatus(t, tidepost, "plot", "episodes", "not_applicable")
	assertRatio(t, tidepost, "scenes", "scenes", 2, 2, "completed")
	assertRatio(t, tidepost, "world", "locations", 2, 2, "completed")
	assertRatio(t, tidepost, "images", "main_visuals", 1, 1, "completed")
	assertRatio(t, tidepost, "images", "portraits", 3, 3, "completed")
	assertRatio(t, tidepost, "images", "chapter_images", 4, 4, "completed")
	assertNoData(t, tidepost, "timeline", "chapter_links")

	if _, err := db.Exec(ctx, sqltest.SeedCompletenessCatalogOnly); err != nil {
		t.Fatalf("seed catalog-only works: %v", err)
	}
	page := getMap(t, router, "/api/v1/admin/knowledge/ops/knowledge/works?q=phase2-catalog&has_knowledge_version=true&limit=10")
	if items := page["items"].([]any); len(items) != 0 {
		t.Fatalf("default work filter returned catalog-only rows: %+v", items)
	}
	page = getMap(t, router, "/api/v1/admin/knowledge/ops/knowledge/works?q=phase2-catalog&has_knowledge_version=all&limit=10")
	items := page["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("all catalog filter returned %d rows, want 2", len(items))
	}
	for _, raw := range items {
		row := raw.(map[string]any)
		if row["has_knowledge_version"] != false || row["completeness"] != nil || row["release_versions"] != float64(0) {
			t.Errorf("catalog-only row fabricated knowledge values: %+v", row)
		}
	}
	unknownType := getMap(t, router, "/api/v1/admin/knowledge/ops/knowledge/works?q=phase2-catalog&has_knowledge_version=all&type=UNKNOWN&limit=10")["items"].([]any)
	if len(unknownType) != 1 || unknownType[0].(map[string]any)["type"] != "UNKNOWN" {
		t.Fatalf("UNKNOWN work type filter returned %+v", unknownType)
	}
	nullTypeRows := getMap(t, router, "/api/v1/admin/knowledge/ops/knowledge/works?q=phase2-catalog&has_knowledge_version=all&type=__null__&limit=10")["items"].([]any)
	if len(nullTypeRows) != 1 || nullTypeRows[0].(map[string]any)["type"] != nil {
		t.Fatalf("NULL work type filter returned %+v", nullTypeRows)
	}
	searched := getMap(t, router, "/api/v1/admin/knowledge/ops/knowledge/works?q=cowboy&has_knowledge_version=true&limit=10")["items"].([]any)
	if len(searched) != 1 || searched[0].(map[string]any)["world_slug"] != "cowboy-bebop-tv" {
		t.Fatalf("work search returned %+v", searched)
	}
	typed := getMap(t, router, "/api/v1/admin/knowledge/ops/knowledge/works?type=SHORT_STORY_COLLECTION&has_knowledge_version=true&limit=10")["items"].([]any)
	if len(typed) != 1 || typed[0].(map[string]any)["world_slug"] != "tidepost" {
		t.Fatalf("work type filter returned %+v", typed)
	}
	partial := getMap(t, router, "/api/v1/admin/knowledge/ops/knowledge/works?completeness_status=partial&has_knowledge_version=true&limit=10")["items"].([]any)
	if len(partial) != 1 || partial[0].(map[string]any)["world_slug"] != "cowboy-bebop-tv" {
		t.Fatalf("completeness status filter returned %+v", partial)
	}
	idle := getMap(t, router, "/api/v1/admin/knowledge/ops/knowledge/works?production_status=idle&has_knowledge_version=true&limit=10")["items"].([]any)
	if len(idle) != 2 {
		t.Fatalf("production status filter returned %d rows, want 2", len(idle))
	}
	firstPage := getMap(t, router, "/api/v1/admin/knowledge/ops/knowledge/works?q=phase2-catalog&has_knowledge_version=all&sort=title&limit=1")
	firstItems := firstPage["items"].([]any)
	if len(firstItems) != 1 || firstItems[0].(map[string]any)["title"] != "phase2-catalog-null" || firstPage["next_cursor"] == nil {
		t.Fatalf("first sorted catalog page=%+v", firstPage)
	}
	cursor := firstPage["next_cursor"].(string)
	nextPath := "/api/v1/admin/knowledge/ops/knowledge/works?q=phase2-catalog&has_knowledge_version=all&sort=title&limit=1&after=" + cursor
	nextPage := getMap(t, router, nextPath)["items"].([]any)
	repeatedNextPage := getMap(t, router, nextPath)["items"].([]any)
	if len(nextPage) != 1 || nextPage[0].(map[string]any)["title"] != "phase2-catalog-unknown" ||
		nextPage[0].(map[string]any)["work_id"] != repeatedNextPage[0].(map[string]any)["work_id"] {
		t.Fatalf("cursor ordering was not stable: first=%+v next=%+v repeated=%+v", firstPage, nextPage, repeatedNextPage)
	}
	if _, err := db.Exec(ctx, sqltest.SetCowboyType, "UNKNOWN"); err != nil {
		t.Fatal(err)
	}
	if err := domain.RecalculateCompleteness(ctx, db); err != nil {
		t.Fatal(err)
	}
	unknown := getCompleteness(t, router, "/api/v1/admin/knowledge/ops/knowledge/completeness?world=cowboy-bebop-tv")
	assertRatio(t, unknown, "basic_profile", "core_fields", 6, 7, "partial")
	if _, err := db.Exec(ctx, sqltest.SetCowboyType, nil); err != nil {
		t.Fatal(err)
	}
	if err := domain.RecalculateCompleteness(ctx, db); err != nil {
		t.Fatal(err)
	}
	nullType := getCompleteness(t, router, "/api/v1/admin/knowledge/ops/knowledge/completeness?world=cowboy-bebop-tv")
	assertRatio(t, nullType, "basic_profile", "core_fields", 6, 7, "partial")

	summary := getMap(t, router, "/api/v1/admin/knowledge/ops/knowledge/summary?range=7d")
	if summary["definition"] != float64(2) || summary["as_of"] == nil || summary["sources"] == nil || summary["trend"] == nil {
		t.Fatalf("knowledge summary omitted required fields: %+v", summary)
	}
	works := summary["works"].(map[string]any)
	var expectedRaw []byte
	if err := db.QueryRow(ctx, sqltest.OpsKnowledgeExpectedSummary).Scan(&expectedRaw); err != nil {
		t.Fatal(err)
	}
	var expected map[string]int
	if err := json.Unmarshal(expectedRaw, &expected); err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]int{"active_total": expected["active_works"], "published_worlds": expected["published_worlds"], "release_versions": expected["release_versions"]} {
		if works[key] != float64(want) {
			t.Errorf("summary works.%s=%v, want %d", key, works[key], want)
		}
	}
	people := summary["people"].(map[string]any)
	plot := summary["plot"].(map[string]any)
	world := summary["world"].(map[string]any)
	images := summary["images"].(map[string]any)
	for key, want := range map[string]int{"total": expected["people"]} {
		if people[key] != float64(want) {
			t.Errorf("summary people.%s=%v, want %d", key, people[key], want)
		}
	}
	for key, want := range map[string]int{"storylines": expected["storylines"], "chapters": expected["chapters"], "episodes_expected": expected["episodes_expected"], "episodes_covered": expected["episodes_covered"], "scenes": expected["scenes"]} {
		if plot[key] != float64(want) {
			t.Errorf("summary plot.%s=%v, want %d", key, plot[key], want)
		}
	}
	if world["locations"] != float64(expected["locations"]) {
		t.Errorf("summary locations=%v want %d", world["locations"], expected["locations"])
	}
	for key, want := range map[string]int{"main_visuals": expected["main_visuals"], "portraits": expected["portraits"], "chapter_images": expected["chapter_images"]} {
		if images[key] != float64(want) {
			t.Errorf("summary images.%s=%v want %d", key, images[key], want)
		}
	}
	if works["by_type"] == nil {
		t.Fatal("summary omitted type distribution")
	}
	types := works["by_type"].([]any)
	if !hasTypeCount(types, "UNKNOWN") || !hasTypeCount(types, nil) {
		t.Fatalf("type distribution did not preserve UNKNOWN and NULL: %+v", types)
	}
	if _, err := db.Exec(ctx, sqltest.SeedPrivateMediaCandidate); err != nil {
		t.Fatal(err)
	}
	workDetail := getMap(t, router, "/api/v1/admin/knowledge/ops/knowledge/works/cowboy-bebop-tv")
	if workDetail["overview"].(map[string]any)["world_slug"] != "cowboy-bebop-tv" || workDetail["scenes"] == nil || workDetail["quality_report"] == nil {
		t.Fatalf("work detail response omitted sections: %+v", workDetail)
	}
	serializedDetail, err := json.Marshal(workDetail)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(serializedDetail), "/media/phase2-private.png") {
		t.Fatal("work detail exposed withdrawn private candidate media")
	}
	workID := workDetail["overview"].(map[string]any)["work_id"].(string)
	byID := getMap(t, router, "/api/v1/admin/knowledge/ops/knowledge/works/"+workID)
	if byID["overview"].(map[string]any)["world_slug"] != "cowboy-bebop-tv" {
		t.Fatalf("UUID work lookup resolved the wrong work: %+v", byID["overview"])
	}
}

func TestCompletenessRuleBoundariesAndEvidence(t *testing.T) {
	db := testsupport.Database(t)
	ctx := context.Background()
	for _, statement := range []string{sqltest.SeedOpsSyncRuns, sqltest.SeedOpsWorlds, sqltest.SeedCompletenessExamples} {
		if _, err := db.Exec(ctx, statement); err != nil {
			t.Fatalf("seed completeness examples: %v", err)
		}
	}
	if err := domain.RecalculateCompleteness(ctx, db); err != nil {
		t.Fatal(err)
	}
	router := completenessRouter(t, db)
	result := getCompleteness(t, router, "/api/v1/admin/knowledge/ops/knowledge/completeness?world=cowboy-bebop-tv")

	if _, err := db.Exec(ctx, sqltest.AddWhitespaceCompletenessLocation); err != nil {
		t.Fatal(err)
	}
	if err := domain.RecalculateCompleteness(ctx, db); err != nil {
		t.Fatal(err)
	}
	result = getCompleteness(t, router, "/api/v1/admin/knowledge/ops/knowledge/completeness?world=cowboy-bebop-tv")
	assertRatio(t, result, "world", "locations", 10, 11, "partial")
	if _, err := db.Exec(ctx, sqltest.RemoveWhitespaceCompletenessLocation); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, sqltest.DuplicateCompletenessLocation); err != nil {
		t.Fatal(err)
	}
	if err := domain.RecalculateCompleteness(ctx, db); err != nil {
		t.Fatal(err)
	}
	result = getCompleteness(t, router, "/api/v1/admin/knowledge/ops/knowledge/completeness?world=cowboy-bebop-tv")
	assertRatio(t, result, "world", "locations", 9, 9, "completed")
	if _, err := db.Exec(ctx, sqltest.RestoreCompletenessLocation); err != nil {
		t.Fatal(err)
	}

	if _, err := db.Exec(ctx, sqltest.SeedCompletenessPendingEvidence); err != nil {
		t.Fatal(err)
	}
	if err := domain.RecalculateCompleteness(ctx, db); err != nil {
		t.Fatal(err)
	}
	result = getCompleteness(t, router, "/api/v1/admin/knowledge/ops/knowledge/completeness?world=cowboy-bebop-tv")
	assertRatio(t, result, "plot", "episodes", 10, 26, "pending_verification")
	if got := detailStatus(t, result, "plot", "episodes", "11"); got != "pending_verification" {
		t.Fatalf("episode 11 status=%q, want pending_verification", got)
	}
	if got := detailStatus(t, result, "plot", "episodes", "12"); got != "missing" {
		t.Fatalf("episode 12 status=%q, want missing without source evidence", got)
	}

	if _, err := db.Exec(ctx, sqltest.PublishCompletenessPendingEpisode); err != nil {
		t.Fatal(err)
	}
	if err := domain.RecalculateCompleteness(ctx, db); err != nil {
		t.Fatal(err)
	}
	result = getCompleteness(t, router, "/api/v1/admin/knowledge/ops/knowledge/completeness?world=cowboy-bebop-tv")
	assertRatio(t, result, "plot", "episodes", 11, 26, "partial")
	if got := detailStatus(t, result, "plot", "episodes", "11"); got != "completed" {
		t.Fatalf("published episode 11 status=%q, want completed", got)
	}
}

func TestCompletenessResultsAreVersionScoped(t *testing.T) {
	db := testsupport.Database(t)
	ctx := context.Background()
	for _, statement := range []string{sqltest.SeedOpsSyncRuns, sqltest.SeedOpsWorlds} {
		if _, err := db.Exec(ctx, statement); err != nil {
			t.Fatalf("seed worlds for version test: %v", err)
		}
	}
	if err := domain.RecalculateCompleteness(ctx, db); err != nil {
		t.Fatal(err)
	}
	var oldVersion int64
	if err := db.QueryRow(ctx, sqltest.CowboyPublishedVersion).Scan(&oldVersion); err != nil {
		t.Fatal(err)
	}
	var oldCount int
	if err := db.QueryRow(ctx, sqltest.CompletenessResultCountForVersion, "cowboy-bebop-tv", oldVersion).Scan(&oldCount); err != nil || oldCount != 17 {
		t.Fatalf("old version result count=%d err=%v, want 17", oldCount, err)
	}
	var newVersion int64
	if err := db.QueryRow(ctx, sqltest.CreateCowboyCompletenessVersion).Scan(&newVersion); err != nil {
		t.Fatalf("publish new version fixture: %v", err)
	}
	if err := domain.RecalculateCompleteness(ctx, db); err != nil {
		t.Fatal(err)
	}
	for version, want := range map[int64]int{oldVersion: 17, newVersion: 17} {
		var got int
		if err := db.QueryRow(ctx, sqltest.CompletenessResultCountForVersion, "cowboy-bebop-tv", version).Scan(&got); err != nil || got != want {
			t.Errorf("version %d result count=%d err=%v, want %d", version, got, err, want)
		}
	}
}

func completenessRouter(t *testing.T, db *pgxpool.Pool) *gin.Engine {
	t.Helper()
	t.Setenv("KNOWLEDGE_ADMIN_TOKEN", "12345678901234567890123456789012")
	t.Setenv("KNOWLEDGE_ADMIN_OPERATOR", "fixture")
	router := gin.New()
	routing.Register(router, db)
	request := httptest.NewRequest("GET", "/api/v1/admin/knowledge/ops/knowledge/summary", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != 401 {
		t.Fatalf("knowledge summary without admin token status=%d", recorder.Code)
	}
	return router
}

func getCompleteness(t *testing.T, router *gin.Engine, path string) completenessResponse {
	t.Helper()
	var result completenessResponse
	body := requestKnowledgeJSON(t, router, path)
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("decode completeness: %v: %s", err, body)
	}
	if !result.Available || len(result.Dimensions) == 0 {
		t.Fatalf("completeness unavailable: %+v", result)
	}
	return result
}

func getMap(t *testing.T, router *gin.Engine, path string) map[string]any {
	t.Helper()
	var result map[string]any
	if err := json.Unmarshal(requestKnowledgeJSON(t, router, path), &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func requestKnowledgeJSON(t *testing.T, router *gin.Engine, path string) []byte {
	t.Helper()
	request := httptest.NewRequest("GET", path, nil)
	request.Header.Set("Authorization", "Bearer 12345678901234567890123456789012")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != 200 {
		t.Fatalf("GET %s status=%d body=%s", path, recorder.Code, recorder.Body.String())
	}
	return recorder.Body.Bytes()
}

func assertRatio(t *testing.T, result completenessResponse, dimension, code string, completed, applicable int, status string) {
	t.Helper()
	item := findItem(t, result, dimension, code)
	if item.DataState != "available" || item.Completed == nil || item.Applicable == nil || *item.Completed != completed || *item.Applicable != applicable || item.Status == nil || *item.Status != status {
		t.Fatalf("%s.%s = %+v, want %d/%d %s", dimension, code, item, completed, applicable, status)
	}
}

func assertStatus(t *testing.T, result completenessResponse, dimension, code, status string) {
	t.Helper()
	item := findItem(t, result, dimension, code)
	if item.Status == nil || *item.Status != status {
		t.Fatalf("%s.%s status=%v want=%s", dimension, code, item.Status, status)
	}
	if status == "not_applicable" && (item.DataState != "available" || item.Completed == nil || *item.Completed != 0 || item.Applicable == nil || *item.Applicable != 0) {
		t.Fatalf("%s.%s not-applicable item entered the denominator: %+v", dimension, code, item)
	}
}

func assertNoData(t *testing.T, result completenessResponse, dimension, code string) {
	t.Helper()
	item := findItem(t, result, dimension, code)
	if item.DataState != "no_data" || item.Completed != nil || item.Applicable != nil || item.Status != nil {
		t.Fatalf("%s.%s should be no_data without a ratio: %+v", dimension, code, item)
	}
}

func findItem(t *testing.T, result completenessResponse, dimension, code string) completenessItem {
	t.Helper()
	for _, group := range result.Dimensions {
		if group.Key == dimension {
			item, ok := group.Items[code]
			if !ok {
				t.Fatalf("missing completeness item %s.%s", dimension, code)
			}
			return item
		}
	}
	t.Fatalf("missing completeness dimension %s", dimension)
	return completenessItem{}
}

func countDetails(result completenessResponse, dimension, code, status string) int {
	count := 0
	for _, group := range result.Dimensions {
		if group.Key != dimension {
			continue
		}
		for _, detail := range group.Items[code].Details {
			if detail.Status == status {
				count++
			}
		}
	}
	return count
}

func detailStatus(t *testing.T, result completenessResponse, dimension, code, itemKey string) string {
	t.Helper()
	item := findItem(t, result, dimension, code)
	for _, detail := range item.Details {
		if detail.ItemKey == itemKey {
			return detail.Status
		}
	}
	t.Fatalf("%s.%s detail %q not found", dimension, code, itemKey)
	return ""
}

func hasTypeCount(rows []any, target any) bool {
	for _, raw := range rows {
		row := raw.(map[string]any)
		if row["type"] == target {
			return true
		}
	}
	return false
}
