package fixtures

import _ "embed"

//go:embed migrations/backup_attempt_definition.sql
var BackupAttemptDefinition string

//go:embed migrations/drop_backup_attempts.sql
var DropBackupAttempts string

//go:embed migrations/applied_at049.sql
var AppliedAt049 string

//go:embed migrations/forget049.sql
var Forget049 string

//go:embed migrations/count049.sql
var Count049 string
