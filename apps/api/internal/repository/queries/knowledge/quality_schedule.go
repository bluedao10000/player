package knowledge

import _ "embed"
import resources "anime-agent/api/internal/repository/queries"

//go:embed quality_schedule.queries
var qualityscheduleSource string

var (
	qualityscheduleSourceQueries = resources.Statements(qualityscheduleSource)
	TryLockQuality               = qualityscheduleSourceQueries["TryLockQuality"]
	FailInterruptedQuality       = qualityscheduleSourceQueries["FailInterruptedQuality"]
	BeginQualityIfDue            = qualityscheduleSourceQueries["BeginQualityIfDue"]
	FailQuality                  = qualityscheduleSourceQueries["FailQuality"]
	FinishQuality                = qualityscheduleSourceQueries["FinishQuality"]
)
