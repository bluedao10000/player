package matching

import _ "embed"
import resources "anime-agent/api/internal/repository/queries"

//go:embed assessment.queries
var assessmentSource string

var (
	assessmentSourceQueries = resources.Statements(assessmentSource)
	MatchEvidence           = assessmentSourceQueries["MatchEvidence"]
	TryLockAssessment       = assessmentSourceQueries["TryLockAssessment"]
	CachedAssessment        = assessmentSourceQueries["CachedAssessment"]
	SaveAssessment          = assessmentSourceQueries["SaveAssessment"]
)
