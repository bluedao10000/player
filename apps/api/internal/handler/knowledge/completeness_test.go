package knowledge_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	domain "anime-agent/api/internal/service/knowledge"
	"anime-agent/api/internal/testsupport"
	sqltest "anime-agent/api/internal/testsupport/fixtures"
)

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

func TestCompletenessExamples(t *testing.T) {
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

	cowboy := getCompleteness(t, db, "cowboy-bebop-tv")
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

	tidepost := getCompleteness(t, db, "tidepost")
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

	if _, err := db.Exec(ctx, sqltest.SetCowboyType, "UNKNOWN"); err != nil {
		t.Fatal(err)
	}
	if err := domain.RecalculateCompleteness(ctx, db); err != nil {
		t.Fatal(err)
	}
	unknown := getCompleteness(t, db, "cowboy-bebop-tv")
	assertRatio(t, unknown, "basic_profile", "core_fields", 6, 7, "partial")
	if _, err := db.Exec(ctx, sqltest.SetCowboyType, nil); err != nil {
		t.Fatal(err)
	}
	if err := domain.RecalculateCompleteness(ctx, db); err != nil {
		t.Fatal(err)
	}
	nullType := getCompleteness(t, db, "cowboy-bebop-tv")
	assertRatio(t, nullType, "basic_profile", "core_fields", 6, 7, "partial")

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
	result := getCompleteness(t, db, "cowboy-bebop-tv")

	if _, err := db.Exec(ctx, sqltest.AddWhitespaceCompletenessLocation); err != nil {
		t.Fatal(err)
	}
	if err := domain.RecalculateCompleteness(ctx, db); err != nil {
		t.Fatal(err)
	}
	result = getCompleteness(t, db, "cowboy-bebop-tv")
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
	result = getCompleteness(t, db, "cowboy-bebop-tv")
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
	result = getCompleteness(t, db, "cowboy-bebop-tv")
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
	result = getCompleteness(t, db, "cowboy-bebop-tv")
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

type completenessResponse map[string]map[string]completenessItem

func getCompleteness(t *testing.T, db *pgxpool.Pool, world string) completenessResponse {
	t.Helper()
	var result completenessResponse
	var body []byte
	if err := db.QueryRow(context.Background(), sqltest.ReadCompletenessResults, world).Scan(&body); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(body, &result); err != nil || len(result) == 0 {
		t.Fatalf("decode completeness for %s: %v: %s", world, err, body)
	}
	return result
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
	item, ok := result[dimension][code]
	if !ok {
		t.Fatalf("missing completeness item %s.%s", dimension, code)
	}
	return item
}

func countDetails(result completenessResponse, dimension, code, status string) int {
	count := 0
	for _, detail := range result[dimension][code].Details {
		if detail.Status == status {
			count++
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
