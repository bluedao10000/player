package knowledge

import _ "embed"

//go:embed sync/enabled_sources.sql
var EnabledSources string

//go:embed sync/try_lock_source.sql
var TryLockSource string

//go:embed sync/record_busy_skip.sql
var RecordBusySkip string

//go:embed sync/fail_interrupted_syncs.sql
var FailInterruptedSyncs string

//go:embed sync/clear_failed_staging.sql
var ClearFailedStaging string

//go:embed sync/source_configuration.sql
var SourceConfiguration string

//go:embed sync/start_sync.sql
var StartSync string

//go:embed sync/touch_source.sql
var TouchSource string

//go:embed sync/fail_sync.sql
var FailSync string

//go:embed sync/delete_batch.sql
var DeleteBatch string

//go:embed sync/archived_source_version.sql
var ArchivedSourceVersion string

//go:embed sync/has_archived_version.sql
var HasArchivedVersion string

//go:embed sync/skip_unchanged_source.sql
var SkipUnchangedSource string

//go:embed sync/record_download.sql
var RecordDownload string

//go:embed sync/record_checksum.sql
var RecordChecksum string

//go:embed sync/record_read_count.sql
var RecordReadCount string

//go:embed sync/validate_staging.sql
var ValidateStaging string

//go:embed sync/register_version.sql
var RegisterVersion string

//go:embed sync/bind_snapshot.sql
var BindSnapshot string

//go:embed sync/create_merge_context.sql
var CreateMergeContext string

//go:embed sync/set_merge_context.sql
var SetMergeContext string

//go:embed sync/mark_replay_works.sql
var MarkReplayWorks string

//go:embed sync/finish_sync.sql
var FinishSync string

//go:embed sync/advance_source_version.sql
var AdvanceSourceVersion string
