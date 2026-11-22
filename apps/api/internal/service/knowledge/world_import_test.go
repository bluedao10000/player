package knowledge

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"anime-agent/api/internal/testsupport"
	sqltest "anime-agent/api/internal/testsupport/fixtures"
)

func TestWorldImportSeedsOpsMediaBatchWorlds(t *testing.T) {
	db := testsupport.Database(t)
	ctx := context.Background()

	var mappings int
	if err := db.QueryRow(ctx, sqltest.OpsWorldMappingCount).Scan(&mappings); err != nil || mappings != 0 {
		t.Fatalf("empty migration mappings=%d err=%v", mappings, err)
	}

	const source = "tidepost-original"
	if err := Sweep(ctx, db, source, true, false); err != nil {
		t.Fatalf("import world: %v", err)
	}
	assertTidepostOpsMapping(t, ctx, db)

	// Force the same local manifest through the normal changed-version import path
	// so the world upsert and its idempotent mapping seed both run again.
	if _, err := db.Exec(ctx, sqltest.ForceWorldReimport); err != nil {
		t.Fatal(err)
	}
	if err := Sweep(ctx, db, source, true, false); err != nil {
		t.Fatalf("reimport world: %v", err)
	}
	assertTidepostOpsMapping(t, ctx, db)
}

func assertTidepostOpsMapping(t *testing.T, ctx context.Context, db *pgxpool.Pool) {
	t.Helper()
	var mappings int
	if err := db.QueryRow(ctx, sqltest.OpsWorldMappingCount).Scan(&mappings); err != nil || mappings != 1 {
		t.Fatalf("mapping rows=%d err=%v", mappings, err)
	}
	var tidepost int
	if err := db.QueryRow(ctx, sqltest.TidepostOpsMappingCount).Scan(&tidepost); err != nil || tidepost != 1 {
		t.Fatalf("tidepost mio mapping rows=%d err=%v", tidepost, err)
	}
}
