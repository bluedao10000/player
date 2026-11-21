package fixtures

import _ "embed"
import resources "anime-agent/api/internal/repository/queries"

//go:embed backup.queries
var backupSource string

var (
	backupSourceQueries = resources.Statements(backupSource)
	SeedBackupHistory   = backupSourceQueries["SeedBackupHistory"]
)
