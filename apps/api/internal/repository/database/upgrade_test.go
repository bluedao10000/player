package database_test

import (
	"context"
	"os"
	"sync"
	"testing"

	"anime-agent/api/internal/repository/database"
	"anime-agent/api/internal/testsupport"
	sqltest "anime-agent/database/queries/fixtures"
)

// Compare the new GORM step to the exact DDL previously shipped under 049.sql.
// All destructive setup below runs in testsupport's private temporary schema.
func TestBackupMigrationCompatibility(t *testing.T) {
	db := testsupport.Database(t)
	ctx := context.Background()
	exec := func(query string) {
		t.Helper()
		if _, err := db.Exec(ctx, query); err != nil {
			t.Fatal(err)
		}
	}
	text := func(query string) string {
		t.Helper()
		var value string
		if err := db.QueryRow(ctx, query).Scan(&value); err != nil {
			t.Fatal(err)
		}
		return value
	}

	fresh := text(sqltest.BackupAttemptDefinition)
	legacy, err := os.ReadFile("../../../../../database/migrations/legacy/049.sql")
	if err != nil {
		t.Fatal(err)
	}
	exec(sqltest.DropBackupAttempts)
	exec(string(legacy))
	if old := text(sqltest.BackupAttemptDefinition); old != fresh {
		t.Fatalf("049 changed schema\nlegacy %s\nGORM   %s", old, fresh)
	}

	original := text(sqltest.AppliedAt049)
	reopen := func() error {
		pool, err := database.Open(ctx)
		if err == nil {
			pool.Close()
		}
		return err
	}
	if err := reopen(); err != nil || text(sqltest.AppliedAt049) != original {
		t.Fatal("replayed an applied migration", err)
	}
	exec(sqltest.Forget049)
	t.Setenv("KNOWLEDGE_VERIFY_SCHEMA_ONLY", "true")
	if err := reopen(); err == nil {
		t.Fatal("accepted an incomplete schema")
	}
	t.Setenv("KNOWLEDGE_VERIFY_SCHEMA_ONLY", "false")
	if err := reopen(); err == nil {
		t.Fatal("marked a conflicting migration as successful")
	}
	exec(sqltest.DropBackupAttempts)
	var workers sync.WaitGroup
	for range 2 {
		workers.Go(func() {
			if err := reopen(); err != nil {
				t.Errorf("upgrade from 048: %v", err)
			}
		})
	}
	workers.Wait()
	if text(sqltest.BackupAttemptDefinition) != fresh || text(sqltest.Count049) != "1" {
		t.Fatal("concurrent upgrade did not install 049 exactly once")
	}
}
