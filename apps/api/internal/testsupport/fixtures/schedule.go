package fixtures

import _ "embed"
import resources "anime-agent/api/internal/repository/queries"

//go:embed schedule.queries
var scheduleSource string

var (
	scheduleSourceQueries = resources.Statements(scheduleSource)
	CreateScheduleEdition = scheduleSourceQueries["CreateScheduleEdition"]
	CountAdmissions       = scheduleSourceQueries["CountAdmissions"]
	InsertMissingEpisodes = scheduleSourceQueries["InsertMissingEpisodes"]
	QueueStatus           = scheduleSourceQueries["QueueStatus"]
)
