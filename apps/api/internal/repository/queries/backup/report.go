package backup

import _ "embed"
import resources "anime-agent/api/internal/repository/queries"

//go:embed report.queries
var reportSource string

var (
	reportSourceQueries = resources.Statements(reportSource)
	UpsertRun           = reportSourceQueries["UpsertRun"]
	HasFailedAttempt    = reportSourceQueries["HasFailedAttempt"]
	InsertFailedAttempt = reportSourceQueries["InsertFailedAttempt"]
	Heartbeat           = reportSourceQueries["Heartbeat"]
)
