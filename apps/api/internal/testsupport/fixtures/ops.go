package fixtures

import _ "embed"
import resources "anime-agent/api/internal/repository/queries"

//go:embed ops.queries
var opsSource string

var opsQueries = resources.Statements(opsSource)

var (
	SeedOpsSyncRuns                  = opsQueries["SeedOpsSyncRuns"]
	SeedOpsWorlds                    = opsQueries["SeedOpsWorlds"]
	SeedOpsMediaBatchWorlds          = opsQueries["SeedOpsMediaBatchWorlds"]
	OpsExpectedBatchMappingCount     = opsQueries["OpsExpectedBatchMappingCount"]
	SeedOpsQualityRuns               = opsQueries["SeedOpsQualityRuns"]
	SeedOpsProductionRequests        = opsQueries["SeedOpsProductionRequests"]
	SeedOpsConfigurationRuns         = opsQueries["SeedOpsConfigurationRuns"]
	SeedOpsEpisodeRuns               = opsQueries["SeedOpsEpisodeRuns"]
	SeedOpsPolicyItems               = opsQueries["SeedOpsPolicyItems"]
	SeedOpsPolicyAdmissions          = opsQueries["SeedOpsPolicyAdmissions"]
	SeedOpsSourceCapture             = opsQueries["SeedOpsSourceCapture"]
	SeedOpsEditors                   = opsQueries["SeedOpsEditors"]
	SeedOpsMediaSteps                = opsQueries["SeedOpsMediaSteps"]
	SeedOpsMediaCandidates           = opsQueries["SeedOpsMediaCandidates"]
	SeedOpsBackups                   = opsQueries["SeedOpsBackups"]
	SeedOpsOverviewSnapshot          = opsQueries["SeedOpsOverviewSnapshot"]
	SeedOpsOverviewHeartbeats        = opsQueries["SeedOpsOverviewHeartbeats"]
	SeedOpsOverviewManyOffline       = opsQueries["SeedOpsOverviewManyOffline"]
	SeedOpsOverviewRuns              = opsQueries["SeedOpsOverviewRuns"]
	OpsOverviewExpectedLiveMetrics   = opsQueries["OpsOverviewExpectedLiveMetrics"]
	OpsRunStatusCounts               = opsQueries["OpsRunStatusCounts"]
	OpsWorkItemsByKey                = opsQueries["OpsWorkItemsByKey"]
	OpsWorkItemCounts                = opsQueries["OpsWorkItemCounts"]
	OpsRunBySourceEpisode            = opsQueries["OpsRunBySourceEpisode"]
	OpsRunByMediaSlot                = opsQueries["OpsRunByMediaSlot"]
	OpsBatchWorld                    = opsQueries["OpsBatchWorld"]
	OpsMediaStepMapper               = opsQueries["OpsMediaStepMapper"]
	OpsCaptureResearchDuplicateCount = opsQueries["OpsCaptureResearchDuplicateCount"]
	OpsMediaDuplicateCount           = opsQueries["OpsMediaDuplicateCount"]
	OpsQueueMapper                   = opsQueries["OpsQueueMapper"]
	OpsSetHeartbeat                  = opsQueries["OpsSetHeartbeat"]
	OpsHeartbeatFields               = opsQueries["OpsHeartbeatFields"]
	OpsSnapshotCount                 = opsQueries["OpsSnapshotCount"]
	OpsSnapshotDefinitions           = opsQueries["OpsSnapshotDefinitions"]
	OpsSnapshotExpectedCount         = opsQueries["OpsSnapshotExpectedCount"]
	OpsMigrationState                = opsQueries["OpsMigrationState"]
	OpsExistingMediaCandidateCount   = opsQueries["OpsExistingMediaCandidateCount"]
	ResetOpsMigration059             = opsQueries["ResetOpsMigration059"]
)
