package fixtures

import _ "embed"
import resources "anime-agent/api/internal/repository/queries"

//go:embed snapshot.queries
var snapshotSource string

var (
	snapshotSourceQueries      = resources.Statements(snapshotSource)
	SetSourceURLs              = snapshotSourceQueries["SetSourceURLs"]
	FindSharedWork             = snapshotSourceQueries["FindSharedWork"]
	CountActiveWorks           = snapshotSourceQueries["CountActiveWorks"]
	CountWorkRecords           = snapshotSourceQueries["CountWorkRecords"]
	CountTitleMatches          = snapshotSourceQueries["CountTitleMatches"]
	WorkTitle                  = snapshotSourceQueries["WorkTitle"]
	CountStaging               = snapshotSourceQueries["CountStaging"]
	CountWorks                 = snapshotSourceQueries["CountWorks"]
	RejectFixtureTitle         = snapshotSourceQueries["RejectFixtureTitle"]
	AllowFixtureTitle          = snapshotSourceQueries["AllowFixtureTitle"]
	LatestChangedCounts        = snapshotSourceQueries["LatestChangedCounts"]
	SetRetention               = snapshotSourceQueries["SetRetention"]
	ActiveWorkTitle            = snapshotSourceQueries["ActiveWorkTitle"]
	LatestDeletionCount        = snapshotSourceQueries["LatestDeletionCount"]
	CountRunningSyncs          = snapshotSourceQueries["CountRunningSyncs"]
	CreateOrphanRun            = snapshotSourceQueries["CreateOrphanRun"]
	CreateOrphanRecord         = snapshotSourceQueries["CreateOrphanRecord"]
	SyncStatus                 = snapshotSourceQueries["SyncStatus"]
	CountDistinctProviderWorks = snapshotSourceQueries["CountDistinctProviderWorks"]
	CountProviderConflicts     = snapshotSourceQueries["CountProviderConflicts"]
	LockSource                 = snapshotSourceQueries["LockSource"]
	LatestSkipReason           = snapshotSourceQueries["LatestSkipReason"]
	UnlockSession              = snapshotSourceQueries["UnlockSession"]
	CountSyncRuns              = snapshotSourceQueries["CountSyncRuns"]
	AgeLastCheck               = snapshotSourceQueries["AgeLastCheck"]
	FirstMatchRecord           = snapshotSourceQueries["FirstMatchRecord"]
	MatchForRecord             = snapshotSourceQueries["MatchForRecord"]
	WorkBySourceKey            = snapshotSourceQueries["WorkBySourceKey"]
	FirstSourceVersion         = snapshotSourceQueries["FirstSourceVersion"]
	SourceVersion              = snapshotSourceQueries["SourceVersion"]
	FirstMatchID               = snapshotSourceQueries["FirstMatchID"]
	EvidenceCandidate          = snapshotSourceQueries["EvidenceCandidate"]
	CountAssessments           = snapshotSourceQueries["CountAssessments"]
	CountMeteredMatches        = snapshotSourceQueries["CountMeteredMatches"]
	CountDecisions             = snapshotSourceQueries["CountDecisions"]
	CountQualityReports        = snapshotSourceQueries["CountQualityReports"]
	AgeQualityRun              = snapshotSourceQueries["AgeQualityRun"]
	CountSuccessfulScans       = snapshotSourceQueries["CountSuccessfulScans"]
	TitleIssueStatus           = snapshotSourceQueries["TitleIssueStatus"]
	SnapshotByVersion          = snapshotSourceQueries["SnapshotByVersion"]
)
