package knowledge

import (
	"context"
	"strings"
	"testing"
)

// The isolated database contains neither the research batch nor any accepted
// scene renders. A world with published pages must still fail all missing gates.
func TestIncompleteEpisodeScopeReportsGapsWithoutPublishing(t *testing.T) {
	db := testStore(t)
	ctx := context.Background()
	if err := Sweep(ctx, db, "tidepost-original", true, false); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, "UPDATE work_worlds SET slug='cowboy-bebop-tv' WHERE slug='tidepost'"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := AcceptEpisodeScope(ctx, db); err != nil {
			t.Fatal(err)
		}
	}
	var count, published int
	var checks []scopeCheck
	if err := db.QueryRow(ctx, "SELECT count(*) FROM scope_acceptances").Scan(&count); err != nil || count != 1 {
		t.Fatalf("duplicate runs: %d %v", count, err)
	}
	if err := db.QueryRow(ctx, "SELECT count(*) FROM published_episode_scopes").Scan(&published); err != nil || published != 0 {
		t.Fatalf("premature release: %d %v", published, err)
	}
	if err := db.QueryRow(ctx, "SELECT checks FROM scope_acceptances WHERE NOT passed AND reader_copy='{}' AND picks='[]'").Scan(&checks); err != nil || len(checks) != 5 {
		t.Fatalf("incomplete report: %v", err)
	}
	for _, c := range checks {
		for _, item := range c.Items {
			if item.Gap == "" {
				t.Fatalf("%s missing gap for %s", c.Name, item.Label)
			}
		}
	}
	if !strings.Contains(checks[1].Items[0].Gap, "正文") || checks[4].Items[9].Label != "街头遛狗" {
		t.Fatal(checks)
	}
}

func TestIncrementalScopeDoesNotReplaceBaseReport(t *testing.T) {
	db := testStore(t)
	ctx := context.Background()
	if err := Sweep(ctx, db, "tidepost-original", true, false); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, "UPDATE work_worlds SET slug='cowboy-bebop-tv' WHERE slug='tidepost'"); err != nil {
		t.Fatal(err)
	}
	if err := AcceptEpisodeScope(ctx, db); err != nil {
		t.Fatal(err)
	}
	var before, after int64
	if err := db.QueryRow(ctx, "SELECT latest FROM scope_heads WHERE topic=$1", scopeTopic).Scan(&before); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := AcceptThirdEpisode(ctx, db); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.QueryRow(ctx, "SELECT latest FROM scope_heads WHERE topic=$1", scopeTopic).Scan(&after); err != nil || before != after {
		t.Fatal(err, before, after)
	}
	var reports int
	var checks []scopeCheck
	if err := db.QueryRow(ctx, "SELECT count(*) FROM scope_acceptances WHERE topic=$1 AND scope_first=3 AND scope_last=3 AND NOT passed", thirdTopic).Scan(&reports); err != nil || reports != 1 {
		t.Fatal(err, reports)
	}
	if err := db.QueryRow(ctx, "SELECT a.checks FROM scope_acceptances a JOIN scope_heads h ON h.latest=a.id WHERE h.published IS NULL AND h.topic=$1", thirdTopic).Scan(&checks); err != nil || len(checks) != 5 {
		t.Fatal(err, checks)
	}
	var visible int
	if err := db.QueryRow(ctx, "SELECT count(*) FROM published_extensions").Scan(&visible); err != nil || visible != 0 {
		t.Fatal(err, visible)
	}
}
