package knowledge_test

import (
	"context"
	"testing"

	"anime-agent/api/internal/testsupport"
	queries "anime-agent/database/queries/release"
)

// Resolve the standalone publisher's SQL against the migrated schema. Both the
// source bundle and published_worlds carry edition information; ambiguous column
// names must be caught before an accepted episode reaches the release timer.
func TestReaderReleaseQueries(t *testing.T) {
	db := testsupport.Database(t)
	ctx := context.Background()
	conn, err := db.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	for query, sql := range map[string]string{"export": queries.ExportRelease, "validate": queries.ValidateRelease, "import": queries.ImportRelease} {
		if _, err = conn.Conn().Prepare(ctx, "reader_release_"+query, sql); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
	}
}
