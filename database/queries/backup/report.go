package backup

import _ "embed"

//go:embed report/upsert_run.sql
var UpsertRun string

//go:embed report/has_failed_attempt.sql
var HasFailedAttempt string

//go:embed report/insert_failed_attempt.sql
var InsertFailedAttempt string

//go:embed report/heartbeat.sql
var Heartbeat string
