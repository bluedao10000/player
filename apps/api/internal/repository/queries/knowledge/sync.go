package knowledge

import _ "embed"
import resources "anime-agent/api/internal/repository/queries"

//go:embed sync.queries
var syncSource string

var (
	syncSourceQueries     = resources.Statements(syncSource)
	EnabledSources        = syncSourceQueries["EnabledSources"]
	TryLockSource         = syncSourceQueries["TryLockSource"]
	RecordBusySkip        = syncSourceQueries["RecordBusySkip"]
	FailInterruptedSyncs  = syncSourceQueries["FailInterruptedSyncs"]
	ClearFailedStaging    = syncSourceQueries["ClearFailedStaging"]
	SourceConfiguration   = syncSourceQueries["SourceConfiguration"]
	StartSync             = syncSourceQueries["StartSync"]
	TouchSource           = syncSourceQueries["TouchSource"]
	FailSync              = syncSourceQueries["FailSync"]
	DeleteBatch           = syncSourceQueries["DeleteBatch"]
	ArchivedSourceVersion = syncSourceQueries["ArchivedSourceVersion"]
	HasArchivedVersion    = syncSourceQueries["HasArchivedVersion"]
	SkipUnchangedSource   = syncSourceQueries["SkipUnchangedSource"]
	RecordDownload        = syncSourceQueries["RecordDownload"]
	RecordChecksum        = syncSourceQueries["RecordChecksum"]
	RecordReadCount       = syncSourceQueries["RecordReadCount"]
	ValidateStaging       = syncSourceQueries["ValidateStaging"]
	RegisterVersion       = syncSourceQueries["RegisterVersion"]
	BindSnapshot          = syncSourceQueries["BindSnapshot"]
	CreateMergeContext    = syncSourceQueries["CreateMergeContext"]
	SetMergeContext       = syncSourceQueries["SetMergeContext"]
	MarkReplayWorks       = syncSourceQueries["MarkReplayWorks"]
	FinishSync            = syncSourceQueries["FinishSync"]
	AdvanceSourceVersion  = syncSourceQueries["AdvanceSourceVersion"]
)
