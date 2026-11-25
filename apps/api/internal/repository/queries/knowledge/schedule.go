package knowledge

import _ "embed"
import resources "anime-agent/api/internal/repository/queries"

//go:embed schedule.queries
var scheduleSource string

var (
	CompleteReleasedPlans = scheduleSourceQueries["CompleteReleasedPlans"]
	ResumeBudgetWait      = scheduleSourceQueries["ResumeBudgetWait"]
	scheduleSourceQueries = resources.Statements(scheduleSource)
	ReserveSubject        = scheduleSourceQueries["ReserveSubject"]
	ClaimPlanDraft        = scheduleSourceQueries["ClaimPlanDraft"]
	MarkPlanDrafting      = scheduleSourceQueries["MarkPlanDrafting"]
	SettlePlanDraft       = scheduleSourceQueries["SettlePlanDraft"]
	SavePlanDraft         = scheduleSourceQueries["SavePlanDraft"]
	InspectPlanInventory  = scheduleSourceQueries["InspectPlanInventory"]
	MatchEdition          = scheduleSourceQueries["MatchEdition"]
	SavePolicy            = scheduleSourceQueries["SavePolicy"]
	PauseUnadmittedTopics = scheduleSourceQueries["PauseUnadmittedTopics"]
	AdmitEpisode          = scheduleSourceQueries["AdmitEpisode"]
	LockCaptureQueue      = scheduleSourceQueries["LockCaptureQueue"]
	ExpireCaptureLeases   = scheduleSourceQueries["ExpireCaptureLeases"]
	ClaimCaptureLease     = scheduleSourceQueries["ClaimCaptureLease"]
)
