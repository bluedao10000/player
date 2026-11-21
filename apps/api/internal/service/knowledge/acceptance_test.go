package knowledge

import (
	"context"
	"strings"
	"testing"

	"anime-agent/api/internal/testsupport"
	sqltest "anime-agent/database/queries/fixtures"
)

// The isolated database contains neither the research batch nor any accepted
// scene renders. A world with published pages must still fail all missing gates.
func TestIncompleteEpisodeScopeReportsGapsWithoutPublishing(t *testing.T) {
	db := testsupport.Database(t)
	ctx := context.Background()
	if err := Sweep(ctx, db, "tidepost-original", true, false); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, sqltest.RenameTestWorld); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := AcceptEpisodeScope(ctx, db); err != nil {
			t.Fatal(err)
		}
	}
	var count, published int
	var checks []scopeCheck
	if err := db.QueryRow(ctx, sqltest.CountAcceptances).Scan(&count); err != nil || count != 1 {
		t.Fatalf("duplicate runs: %d %v", count, err)
	}
	if err := db.QueryRow(ctx, sqltest.CountPublishedScopes).Scan(&published); err != nil || published != 0 {
		t.Fatalf("premature release: %d %v", published, err)
	}
	if err := db.QueryRow(ctx, sqltest.UnpublishedChecks).Scan(&checks); err != nil || len(checks) != 5 {
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
	db := testsupport.Database(t)
	ctx := context.Background()
	if err := Sweep(ctx, db, "tidepost-original", true, false); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, sqltest.RenameTestWorld); err != nil {
		t.Fatal(err)
	}
	if err := AcceptEpisodeScope(ctx, db); err != nil {
		t.Fatal(err)
	}
	var before, after int64
	if err := db.QueryRow(ctx, sqltest.LatestScopeReport, scopeTopic).Scan(&before); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := AcceptThirdEpisode(ctx, db); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.QueryRow(ctx, sqltest.LatestScopeReport, scopeTopic).Scan(&after); err != nil || before != after {
		t.Fatal(err, before, after)
	}
	var reports int
	var checks []scopeCheck
	if err := db.QueryRow(ctx, sqltest.CountEpisodeThreeFailures, thirdTopic).Scan(&reports); err != nil || reports != 1 {
		t.Fatal(err, reports)
	}
	if err := db.QueryRow(ctx, sqltest.OutstandingScopeChecks, thirdTopic).Scan(&checks); err != nil || len(checks) != 5 {
		t.Fatal(err, checks)
	}
	var visible int
	if err := db.QueryRow(ctx, sqltest.CountPublishedExtensions).Scan(&visible); err != nil || visible != 0 {
		t.Fatal(err, visible)
	}
}
