package database_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"anime-agent/api/internal/repository/queries/knowledge"
	"anime-agent/api/internal/repository/queries/reader"
	"anime-agent/api/internal/repository/queries/release"
	"anime-agent/api/internal/testsupport"
	"anime-agent/api/internal/testsupport/fixtures"
)

func TestAdvisoryLockNamespace(t *testing.T) {
	db := testsupport.Database(t)
	q := fixtures.AdvisoryLocks
	for _, tc := range []struct {
		name, namespace, holder, contender string
		blocks                             bool
	}{
		{"old issuer blocks default", "", q["LegacyCodeIssuer"], reader.LockCodeIssuer, true},
		{"old issuer blocks unknown", "v3", q["LegacyCodeIssuer"], reader.LockCodeIssuer, true},
		{"v2 issuer uses reserved key", "v2", q["V2CodeIssuer"], reader.LockCodeIssuer, true},
		{"v2 issuers share quota", "v2", reader.LockCodeIssuer, reader.LockCodeIssuer, true},
		{"v2 issuer independent of projection", "v2", knowledge.LockProjection, reader.LockCodeIssuer, false},
		{"v2 projection retains legacy key", "v2", q["LegacyCodeIssuer"], knowledge.LockProjection, true},
		{"v2 projection and curation serialize", "v2", knowledge.LockProjection, knowledge.BeginCuration, true},
		{"v2 receiver retains legacy key", "v2", q["LegacyPublication"], release.LockPublication, true},
		{"v2 receiver and registration serialize", "v2", knowledge.LockPlanRegistration, release.LockPublication, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			begin := func() pgx.Tx {
				t.Helper()
				tx, err := db.Begin(ctx)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
				if tc.namespace != "" {
					if _, err := tx.Exec(ctx, q["SetNamespace"], tc.namespace); err != nil {
						t.Fatal(err)
					}
				}
				return tx
			}
			holder, contender := begin(), begin()
			if _, err := holder.Exec(ctx, tc.holder); err != nil {
				t.Fatal(err)
			}
			if _, err := contender.Exec(ctx, q["BoundLockWait"]); err != nil {
				t.Fatal(err)
			}
			_, err := contender.Exec(ctx, tc.contender)
			var pgErr *pgconn.PgError
			if tc.blocks {
				if !errors.As(err, &pgErr) || pgErr.Code != "55P03" {
					t.Fatalf("expected lock timeout, got %v", err)
				}
			} else if err != nil {
				t.Fatalf("independent transaction could not acquire lock: %v", err)
			}
		})
	}
}
