package fixtures

import _ "embed"

//go:embed snapshot/set_source_ur_ls.sql
var SetSourceURLs string

//go:embed snapshot/find_shared_work.sql
var FindSharedWork string

//go:embed snapshot/count_active_works.sql
var CountActiveWorks string

//go:embed snapshot/count_work_records.sql
var CountWorkRecords string

//go:embed snapshot/count_title_matches.sql
var CountTitleMatches string

//go:embed snapshot/work_title.sql
var WorkTitle string

//go:embed snapshot/count_staging.sql
var CountStaging string

//go:embed snapshot/count_works.sql
var CountWorks string

//go:embed snapshot/reject_fixture_title.sql
var RejectFixtureTitle string

//go:embed snapshot/allow_fixture_title.sql
var AllowFixtureTitle string

//go:embed snapshot/latest_changed_counts.sql
var LatestChangedCounts string

//go:embed snapshot/set_retention.sql
var SetRetention string

//go:embed snapshot/active_work_title.sql
var ActiveWorkTitle string

//go:embed snapshot/latest_deletion_count.sql
var LatestDeletionCount string

//go:embed snapshot/count_running_syncs.sql
var CountRunningSyncs string

//go:embed snapshot/create_orphan_run.sql
var CreateOrphanRun string

//go:embed snapshot/create_orphan_record.sql
var CreateOrphanRecord string

//go:embed snapshot/sync_status.sql
var SyncStatus string

//go:embed snapshot/count_distinct_provider_works.sql
var CountDistinctProviderWorks string

//go:embed snapshot/count_provider_conflicts.sql
var CountProviderConflicts string

//go:embed snapshot/lock_source.sql
var LockSource string

//go:embed snapshot/latest_skip_reason.sql
var LatestSkipReason string

//go:embed snapshot/unlock_session.sql
var UnlockSession string

//go:embed snapshot/count_sync_runs.sql
var CountSyncRuns string

//go:embed snapshot/age_last_check.sql
var AgeLastCheck string

//go:embed snapshot/first_match_record.sql
var FirstMatchRecord string

//go:embed snapshot/match_for_record.sql
var MatchForRecord string

//go:embed snapshot/work_by_source_key.sql
var WorkBySourceKey string

//go:embed snapshot/first_source_version.sql
var FirstSourceVersion string

//go:embed snapshot/source_version.sql
var SourceVersion string

//go:embed snapshot/first_match_id.sql
var FirstMatchID string

//go:embed snapshot/evidence_candidate.sql
var EvidenceCandidate string

//go:embed snapshot/count_assessments.sql
var CountAssessments string

//go:embed snapshot/count_metered_matches.sql
var CountMeteredMatches string

//go:embed snapshot/count_decisions.sql
var CountDecisions string

//go:embed snapshot/count_quality_reports.sql
var CountQualityReports string

//go:embed snapshot/age_quality_run.sql
var AgeQualityRun string

//go:embed snapshot/count_successful_scans.sql
var CountSuccessfulScans string

//go:embed snapshot/title_issue_status.sql
var TitleIssueStatus string

//go:embed snapshot/snapshot_by_version.sql
var SnapshotByVersion string
