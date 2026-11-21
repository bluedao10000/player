package fixtures

import _ "embed"
import resources "anime-agent/api/internal/repository/queries"

//go:embed migrations.queries
var migrationsSource string

var (
	migrationsSourceQueries = resources.Statements(migrationsSource)
	BackupAttemptDefinition = migrationsSourceQueries["BackupAttemptDefinition"]
	DropBackupAttempts      = migrationsSourceQueries["DropBackupAttempts"]
	AppliedAt049            = migrationsSourceQueries["AppliedAt049"]
	Forget049               = migrationsSourceQueries["Forget049"]
	Count049                = migrationsSourceQueries["Count049"]
)
