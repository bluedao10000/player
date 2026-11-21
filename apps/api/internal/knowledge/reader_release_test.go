package knowledge

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// Resolve the standalone publisher's SQL against the migrated schema. Both the
// source bundle and published_worlds carry edition information; ambiguous column
// names must be caught before an accepted episode reaches the release timer.
func TestReaderReleaseQueries(t *testing.T) {
	db := testStore(t)
	ctx := context.Background()
	conn, err := db.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	for _, query := range []string{"export", "validate", "import"} {
		sql, err := os.ReadFile(filepath.Join("..", "..", "cmd", "reader-release", query+".sql"))
		if err != nil {
			t.Fatal(err)
		}
		if _, err = conn.Conn().Prepare(ctx, "reader_release_"+query, string(sql)); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
	}
}
