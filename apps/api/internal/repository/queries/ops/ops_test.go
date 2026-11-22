package ops_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"anime-agent/api/internal/repository/database"
	ops "anime-agent/api/internal/repository/queries/ops"
	domain "anime-agent/api/internal/service/knowledge"
	"anime-agent/api/internal/testsupport"
	sqltest "anime-agent/api/internal/testsupport/fixtures"
)

func TestOpsMigrationFreshExistingDataAndRepeat(t *testing.T) {
	db := testsupport.Database(t)
	ctx := context.Background()
	assertMigrationState := func(expectedBatches int64) {
		t.Helper()
		var applied, stages, batches, statusCodeColumns int64
		if err := db.QueryRow(ctx, sqltest.OpsMigrationState).Scan(&applied, &stages, &batches, &statusCodeColumns); err != nil {
			t.Fatal(err)
		}
		if applied != 2 || stages != 17 || batches != expectedBatches || statusCodeColumns != 1 {
			t.Fatalf("migration state applied=%d stages=%d batches=%d status_code columns=%d", applied, stages, batches, statusCodeColumns)
		}
	}
	assertMigrationState(0) // A fresh isolated schema has no world rows to satisfy the batch FK.
	for _, statement := range []string{sqltest.SeedOpsSyncRuns, sqltest.SeedOpsWorlds} {
		if _, err := db.Exec(ctx, statement); err != nil {
			t.Fatalf("seed existing source data: %v", err)
		}
	}
	var expectedBatches int64
	if err := db.QueryRow(ctx, sqltest.OpsExpectedBatchMappingCount).Scan(&expectedBatches); err != nil {
		t.Fatal(err)
	}
	var candidatesBefore int
	if err := db.QueryRow(ctx, sqltest.OpsExistingMediaCandidateCount).Scan(&candidatesBefore); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, sqltest.ResetOpsMigration059); err != nil {
		t.Fatalf("reset migrations 059/060 objects: %v", err)
	}
	reopened, err := database.Open(ctx)
	if err != nil {
		t.Fatalf("apply migrations 059/060 over existing data: %v", err)
	}
	reopened.Close()
	assertMigrationState(expectedBatches)

	for batch, expected := range map[string]string{
		"mio":            "tidepost",
		"bebop":          "cowboy-bebop-tv",
		"bebop-portrait": "cowboy-bebop-tv",
	} {
		var world string
		if err := db.QueryRow(ctx, sqltest.OpsBatchWorld, batch).Scan(&world); err != nil || world != expected {
			t.Fatalf("batch %q = %q, err=%v", batch, world, err)
		}
	}

	// Reopening an already-upgraded schema does not duplicate initial rows.
	reopened, err = database.Open(ctx)
	if err != nil {
		t.Fatalf("repeat migration: %v", err)
	}
	reopened.Close()
	assertMigrationState(expectedBatches)
	var candidatesAfter int
	if err := db.QueryRow(ctx, sqltest.OpsExistingMediaCandidateCount).Scan(&candidatesAfter); err != nil {
		t.Fatal(err)
	}
	if candidatesAfter != candidatesBefore {
		t.Fatalf("existing media candidates changed from %d to %d", candidatesBefore, candidatesAfter)
	}
}

func TestOpsSourceAndWorkItemMappings(t *testing.T) {
	db := testsupport.Database(t)
	ctx := context.Background()
	for _, statement := range []string{
		sqltest.SeedOpsSyncRuns,
		sqltest.SeedOpsWorlds,
		sqltest.SeedOpsMediaBatchWorlds,
		sqltest.SeedOpsQualityRuns,
		sqltest.SeedOpsProductionRequests,
		sqltest.SeedOpsConfigurationRuns,
		sqltest.SeedOpsEpisodeRuns,
		sqltest.SeedOpsPolicyItems,
		sqltest.SeedOpsPolicyAdmissions,
		sqltest.SeedOpsSourceCapture,
		sqltest.SeedOpsEditors,
		sqltest.SeedOpsMediaSteps,
		sqltest.SeedOpsMediaCandidates,
		sqltest.SeedOpsBackups,
	} {
		if _, err := db.Exec(ctx, statement); err != nil {
			t.Fatalf("fixture failed: %v\n%s", err, statement)
		}
	}

	type runKey struct{ source, stage, status, outcome string }
	counts := map[runKey]int{}
	rows, err := db.Query(ctx, sqltest.OpsRunStatusCounts)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var key runKey
		var outcome sql.NullString
		var count int
		if err := rows.Scan(&key.source, &key.stage, &key.status, &outcome, &count); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		if outcome.Valid {
			key.outcome = outcome.String
		}
		counts[key] = count
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		t.Fatal(err)
	}
	rows.Close()

	expectedRuns := map[runKey]int{
		{"knowledge_sync_runs", "source_sync", "running", ""}:                          1,
		{"knowledge_sync_runs", "source_sync", "success", ""}:                          1,
		{"knowledge_sync_runs", "source_sync", "skipped", "version_unchanged"}:         1,
		{"knowledge_sync_runs", "source_sync", "failed", ""}:                           1,
		{"knowledge_quality_runs", "quality_scan", "running", ""}:                      1,
		{"knowledge_quality_runs", "quality_scan", "success", ""}:                      1,
		{"knowledge_quality_runs", "quality_scan", "failed", ""}:                       1,
		{"episode_configuration_attempts", "configuration", "running", ""}:             1,
		{"episode_configuration_attempts", "configuration", "failed", ""}:              1,
		{"episode_configuration_attempts", "configuration", "success", "registered"}:   1,
		{"episode_configuration_attempts", "configuration", "skipped", "unregistered"}: 1,
		{"episode_capture_attempts", "capture", "running", ""}:                         1,
		{"episode_capture_attempts", "capture", "success", ""}:                         1,
		{"episode_capture_attempts", "capture", "failed", ""}:                          1,
		{"research_runs", "research", "running", ""}:                                   2,
		{"research_runs", "research", "success", ""}:                                   1,
		{"research_runs", "research", "failed", ""}:                                    1,
		{"editor_tasks", "editorial", "running", ""}:                                   3,
		{"editor_tasks", "editorial", "success", ""}:                                   1,
		{"editor_tasks", "editorial", "failed", ""}:                                    1,
		{"media_steps", "media_reference", "running", ""}:                              1,
		{"media_steps", "media_reference", "success", "pass"}:                          1,
		{"media_steps", "media_reference", "failed", ""}:                               1,
		{"media_steps", "media_generate", "running", ""}:                               1,
		{"media_steps", "media_generate", "success", "pass"}:                           1,
		{"media_steps", "media_generate", "failed", ""}:                                1,
		{"media_steps", "media_review", "running", ""}:                                 1,
		{"media_steps", "media_review", "success", "pass"}:                             1,
		{"media_steps", "media_review", "failed", ""}:                                  1,
		{"media_steps", "media_rework", "running", ""}:                                 2,
		{"media_steps", "media_rework", "success", "pass"}:                             2,
		{"media_steps", "media_rework", "failed", ""}:                                  2,
		{"scope_acceptances", "acceptance", "success", "passed"}:                       4,
		{"scope_acceptances", "acceptance", "success", "rejected"}:                     1,
		{"reader_release_attempts", "release", "running", ""}:                          1,
		{"reader_release_attempts", "release", "success", ""}:                          1,
		{"reader_release_attempts", "release", "failed", ""}:                           1,
		{"backup_runs", "backup", "running", ""}:                                       1,
		{"backup_runs", "backup", "success", ""}:                                       1,
		{"backup_runs", "backup", "failed", ""}:                                        1,
	}
	for key, expected := range expectedRuns {
		if got := counts[key]; got != expected {
			t.Errorf("run mapping %+v = %d, want %d", key, got, expected)
		}
	}
	if counts[runKey{"knowledge_sync_runs", "source_sync", "pending", ""}] != 0 {
		t.Fatal("pending source sync attempt should not be an execution")
	}

	assertRunWorld := func(source string, episode int, stage, status, world string) {
		t.Helper()
		var gotStage, gotStatus, gotWorld string
		var outcome sql.NullString
		var first, last int
		if err := db.QueryRow(ctx, sqltest.OpsRunBySourceEpisode, source, episode).Scan(&gotStage, &gotStatus, &outcome, &gotWorld, &first, &last); err != nil {
			t.Fatal(err)
		}
		if gotStage != stage || gotStatus != status || gotWorld != world || first != episode || last != episode {
			t.Fatalf("run mapping source=%s episode=%d: (%s,%s,%s,%d,%d)", source, episode, gotStage, gotStatus, gotWorld, first, last)
		}
	}
	assertRunWorld("episode_capture_attempts", 901, "capture", "running", "tidepost")

	for batch, expected := range map[string]string{"mio": "tidepost", "bebop": "cowboy-bebop-tv"} {
		var got string
		if err := db.QueryRow(ctx, sqltest.OpsBatchWorld, batch).Scan(&got); err != nil || got != expected {
			t.Fatalf("batch mapping %s = %q, err=%v", batch, got, err)
		}
	}
	for slot, expected := range map[int]string{1: "tidepost", 10: "cowboy-bebop-tv"} {
		rows, err := db.Query(ctx, sqltest.OpsRunByMediaSlot, slot)
		if err != nil {
			t.Fatal(err)
		}
		if !rows.Next() {
			rows.Close()
			t.Fatalf("no media run for slot %d", slot)
		}
		var stage, status, world string
		var outcome sql.NullString
		if err := rows.Scan(&stage, &status, &outcome, &world); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		rows.Close()
		if world != expected || status != "running" {
			t.Fatalf("media slot %d maps to world=%s status=%s", slot, world, status)
		}
	}

	workCounts := map[string]int{}
	workRows, err := db.Query(ctx, sqltest.OpsWorkItemCounts)
	if err != nil {
		t.Fatal(err)
	}
	for workRows.Next() {
		var stage, state, reason string
		var count int
		if err := workRows.Scan(&stage, &state, &reason, &count); err != nil {
			workRows.Close()
			t.Fatal(err)
		}
		workCounts[fmt.Sprintf("%s|%s|%s", stage, state, reason)] += count
	}
	if err := workRows.Err(); err != nil {
		workRows.Close()
		t.Fatal(err)
	}
	workRows.Close()
	for key, expected := range map[string]int{
		"admission|pending|capture_retry":            1,
		"plan|blocked|insufficient_remaining_tokens": 1,
		"plan|blocked|plan_blocked":                  1,
		"plan|pending|planning":                      3,
		"editorial|pending|awaiting_worker":          3,
		"editorial|waiting_human|awaiting_review":    1,
		"editorial|blocked|draft_conflict":           1,
		"media_review|waiting_human|media_held":      1,
	} {
		if workCounts[key] != expected {
			t.Errorf("work item mapping %q = %d, want %d", key, workCounts[key], expected)
		}
	}

	for _, policy := range []struct{ label, code, state, reason string }{
		{"待准入", "awaiting_admission", "pending", "awaiting_admission"},
		{"待采集", "awaiting_capture", "pending", "awaiting_capture"},
		{"已采集", "captured", "pending", "awaiting_research"},
		{"token 预算不足", "token_budget", "blocked", "token_budget"},
		{"集数预算已满", "episode_budget", "blocked", "episode_budget"},
		{"等待有效盘点", "stale_inventory", "blocked", "stale_inventory"},
		{"缺分集配置", "missing_configuration", "blocked", "missing_configuration"},
		{"采集失败", "capture_retry", "pending", "capture_retry"},
	} {
		var state, reason sql.NullString
		if err := db.QueryRow(ctx, sqltest.OpsQueueMapper, policy.code).Scan(&state, &reason); err != nil {
			t.Fatalf("queue mapping %q (%s): %v", policy.label, policy.code, err)
		}
		if !state.Valid || state.String != policy.state || !reason.Valid || reason.String != policy.reason {
			t.Errorf("queue mapping %q (%s) = (%v,%v)", policy.label, policy.code, state, reason)
		}
	}
	for _, terminal := range []string{"passed", "unconfirmed_missing"} {
		var state, reason sql.NullString
		if err := db.QueryRow(ctx, sqltest.OpsQueueMapper, terminal).Scan(&state, &reason); err != nil || state.Valid || reason.Valid {
			t.Errorf("terminal queue mapping %q = (%v,%v), err=%v", terminal, state, reason, err)
		}
	}
	if err := db.QueryRow(ctx, sqltest.OpsQueueMapper, "unknown_status_code").Scan(new(sql.NullString), new(sql.NullString)); err == nil {
		t.Fatal("unmapped episode_policy_queue status_code did not fail")
	}
	var mediaStage string
	if err := db.QueryRow(ctx, sqltest.OpsMediaStepMapper, "recrop").Scan(&mediaStage); err != nil || mediaStage != "media_rework" {
		t.Fatalf("recrop stage = %q, err=%v", mediaStage, err)
	}
	if err := db.QueryRow(ctx, sqltest.OpsMediaStepMapper, "unexpected").Scan(&mediaStage); err == nil {
		t.Fatal("unmapped media step did not fail")
	}

	duplicates, err := db.Query(ctx, sqltest.OpsCaptureResearchDuplicateCount)
	if err != nil {
		t.Fatal(err)
	}
	for duplicates.Next() {
		var count int
		if err := duplicates.Scan(&count); err != nil || count != 0 {
			t.Errorf("capture/research running duplicate count=%d err=%v", count, err)
		}
	}
	if err := duplicates.Err(); err != nil {
		t.Fatal(err)
	}
	duplicates.Close()
	var mediaDuplicates int
	if err := db.QueryRow(ctx, sqltest.OpsMediaDuplicateCount).Scan(&mediaDuplicates); err != nil || mediaDuplicates != 0 {
		t.Fatalf("media running duplicate count=%d err=%v", mediaDuplicates, err)
	}
}

func TestOpsHeartbeatPersistenceAndOnlineBoundaries(t *testing.T) {
	db := testsupport.Database(t)
	ctx := context.Background()
	beat := ops.Heartbeat{WorkerID: "fixture/agent", Loop: "status", Role: "agent", Node: "fixture", Version: "v-test", ExpectedSeconds: 40}
	longError := strings.Repeat("界", 501)
	if err := ops.WriteHeartbeatFailure(ctx, db, beat, longError, 1); err != nil {
		t.Fatal(err)
	}
	var lastTick time.Time
	var lastOK sql.NullTime
	var consecutive int
	var lastError sql.NullString
	var lastErrorAt sql.NullTime
	if err := db.QueryRow(ctx, sqltest.OpsHeartbeatFields, beat.WorkerID, beat.Loop).Scan(&lastTick, &lastOK, &consecutive, &lastError, &lastErrorAt); err != nil {
		t.Fatal(err)
	}
	if consecutive != 1 || utf8.RuneCountInString(lastError.String) != 500 || !lastErrorAt.Valid {
		t.Fatalf("failure heartbeat count=%d error-runes=%d error-at=%t", consecutive, utf8.RuneCountInString(lastError.String), lastErrorAt.Valid)
	}
	if err := ops.WriteHeartbeatFailure(ctx, db, beat, "second failure", 2); err != nil {
		t.Fatal(err)
	}
	if err := ops.WriteHeartbeatSuccess(ctx, db, beat); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(ctx, sqltest.OpsHeartbeatFields, beat.WorkerID, beat.Loop).Scan(&lastTick, &lastOK, &consecutive, &lastError, &lastErrorAt); err != nil {
		t.Fatal(err)
	}
	if consecutive != 0 || !lastOK.Valid {
		t.Fatalf("success did not clear error count: count=%d lastOK=%v", consecutive, lastOK)
	}
	lastTickBeforeFailure, lastOKBeforeFailure := lastTick, lastOK
	if err := ops.WriteAgentHeartbeatFailure(ctx, db, beat, "Agent unavailable", 1); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(ctx, sqltest.OpsHeartbeatFields, beat.WorkerID, beat.Loop).Scan(&lastTick, &lastOK, &consecutive, &lastError, &lastErrorAt); err != nil {
		t.Fatal(err)
	}
	if !lastTick.Equal(lastTickBeforeFailure) || !lastOK.Time.Equal(lastOKBeforeFailure.Time) || consecutive != 1 || lastError.String != "Agent unavailable" {
		t.Fatalf("Agent poll failure changed success timestamps: tick %v/%v ok %v/%v errors %d lastError %q", lastTickBeforeFailure, lastTick, lastOKBeforeFailure, lastOK, consecutive, lastError.String)
	}

	at := time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name       string
		lastOK     time.Time
		errors     int
		wantStatus string
	}{
		{"threshold is online", at.Add(-120 * time.Second), 0, "online"},
		{"past threshold is offline", at.Add(-120*time.Second - time.Microsecond), 0, "offline"},
		{"three errors is online_error", at, 3, "online_error"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := db.Exec(ctx, sqltest.OpsSetHeartbeat, at, tc.lastOK, tc.errors); err != nil {
				t.Fatal(err)
			}
			status, err := ops.WorkerStatusAt(ctx, db, beat.WorkerID, beat.Loop, at)
			if err != nil || status != tc.wantStatus {
				t.Fatalf("status=%q err=%v want=%q", status, err, tc.wantStatus)
			}
		})
	}
}

func TestOpsDailySnapshotIsIdempotent(t *testing.T) {
	db := testsupport.Database(t)
	ctx := context.Background()
	day := time.Date(2026, 11, 22, 0, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	legacyDay := day.AddDate(0, 0, -1)
	for _, statement := range []string{sqltest.SeedOpsSyncRuns, sqltest.SeedOpsWorlds} {
		if _, err := db.Exec(ctx, statement); err != nil {
			t.Fatalf("seed published worlds for snapshot: %v", err)
		}
	}
	if _, err := db.Exec(ctx, sqltest.OpsLegacySnapshot, legacyDay.Format("2006-01-02")); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, sqltest.SeedCompletenessCatalogOnly); err != nil {
		t.Fatal(err)
	}
	if err := domain.RecalculateCompleteness(ctx, db); err != nil {
		t.Fatalf("calculate completeness for snapshot: %v", err)
	}
	if err := ops.TakeDailySnapshot(ctx, db, day); err != nil {
		t.Fatal(err)
	}
	var activeWorksBefore int
	if err := db.QueryRow(ctx, sqltest.OpsSnapshotActiveWorks, day.Format("2006-01-02")).Scan(&activeWorksBefore); err != nil {
		t.Fatal(err)
	}
	var legacyDefinition int
	var legacyMarker sql.NullString
	if err := db.QueryRow(ctx, sqltest.OpsLegacySnapshotState, legacyDay.Format("2006-01-02")).Scan(&legacyDefinition, &legacyMarker); err != nil {
		t.Fatal(err)
	}
	if legacyDefinition != 1 || !legacyMarker.Valid || legacyMarker.String != "true" {
		t.Fatalf("legacy definition 1 snapshot changed: definition=%d marker=%+v", legacyDefinition, legacyMarker)
	}
	if _, err := db.Exec(ctx, sqltest.DeactivateCompletenessCatalogOnly); err != nil {
		t.Fatal(err)
	}
	if err := ops.TakeDailySnapshot(ctx, db, day.Add(12*time.Hour)); err != nil {
		t.Fatal(err)
	}
	var got, definitions int
	var expected int64
	if err := db.QueryRow(ctx, sqltest.OpsSnapshotCount, day.Format("2006-01-02")).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(ctx, sqltest.OpsSnapshotExpectedCount).Scan(&expected); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(ctx, sqltest.OpsSnapshotDefinitions, day.Format("2006-01-02")).Scan(&definitions); err != nil {
		t.Fatal(err)
	}
	if int64(got) != expected || int64(definitions) != expected {
		t.Fatalf("snapshot rows=%d expected=%d definition2=%d", got, expected, definitions)
	}
	var activeWorksAfter int
	if err := db.QueryRow(ctx, sqltest.OpsSnapshotActiveWorks, day.Format("2006-01-02")).Scan(&activeWorksAfter); err != nil || activeWorksAfter != activeWorksBefore {
		t.Fatalf("idempotent snapshot changed active works %d -> %d (err=%v)", activeWorksBefore, activeWorksAfter, err)
	}
	var completenessRaw []byte
	if err := db.QueryRow(ctx, sqltest.OpsSnapshotCompleteness, day.Format("2006-01-02")).Scan(&completenessRaw); err != nil {
		t.Fatal(err)
	}
	var completeness map[string]any
	if err := json.Unmarshal(completenessRaw, &completeness); err != nil || completeness["ruleVersion"] == nil || completeness["dimensions"] == nil {
		t.Fatalf("definition 2 snapshot omitted completeness results: %s (err=%v)", completenessRaw, err)
	}
	var worldDefinition int
	var worldCompletenessRaw []byte
	if err := db.QueryRow(ctx, sqltest.OpsWorldSnapshotCompleteness, day.Format("2006-01-02"), "cowboy-bebop-tv").Scan(&worldDefinition, &worldCompletenessRaw); err != nil {
		t.Fatal(err)
	}
	if worldDefinition != 2 || len(worldCompletenessRaw) == 0 {
		t.Fatalf("world snapshot definition=%d completeness=%s", worldDefinition, worldCompletenessRaw)
	}
}

func TestBeijingSnapshotDayAcrossMidnight(t *testing.T) {
	for _, tc := range []struct {
		utc time.Time
		day string
		due bool
	}{
		{time.Date(2026, 11, 21, 15, 59, 0, 0, time.UTC), "2026-11-21", true},
		{time.Date(2026, 11, 21, 16, 4, 0, 0, time.UTC), "2026-11-22", false},
		{time.Date(2026, 11, 21, 16, 5, 0, 0, time.UTC), "2026-11-22", true},
	} {
		day, due := ops.BeijingSnapshotDay(tc.utc)
		if got := day.Format("2006-01-02"); got != tc.day || due != tc.due {
			t.Errorf("at %s day=%s due=%t; want %s due=%t", tc.utc, got, due, tc.day, tc.due)
		}
	}
}

func TestOpsUnexpectedStatusErrorsAreNotSuppressed(t *testing.T) {
	db := testsupport.Database(t)
	ctx := context.Background()
	var state, reason sql.NullString
	if err := db.QueryRow(ctx, sqltest.OpsQueueMapper, "not-a-policy-status-code").Scan(&state, &reason); err == nil {
		t.Fatal("expected an error for an unmapped queue status_code")
	} else if errors.Is(err, sql.ErrNoRows) {
		t.Fatal("unknown queue state was silently omitted")
	}
}
