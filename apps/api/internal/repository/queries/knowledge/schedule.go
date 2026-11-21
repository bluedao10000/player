package knowledge

import _ "embed"
import resources "anime-agent/api/internal/repository/queries"

//go:embed schedule.queries
var scheduleSource string

var (
	scheduleSourceQueries = resources.Statements(scheduleSource)
	MatchEdition          = scheduleSourceQueries["MatchEdition"]
	SavePolicy            = scheduleSourceQueries["SavePolicy"]
	PauseUnadmittedTopics = scheduleSourceQueries["PauseUnadmittedTopics"]
	AdmitEpisode          = scheduleSourceQueries["AdmitEpisode"]
	LockCaptureQueue      = scheduleSourceQueries["LockCaptureQueue"]
	ExpireCaptureLeases   = scheduleSourceQueries["ExpireCaptureLeases"]
	ClaimCaptureLease     = scheduleSourceQueries["ClaimCaptureLease"]
)
