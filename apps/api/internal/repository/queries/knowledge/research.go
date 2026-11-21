package knowledge

import _ "embed"
import resources "anime-agent/api/internal/repository/queries"

//go:embed research.queries
var researchSource string

var (
	researchSourceQueries = resources.Statements(researchSource)
	TryLockResearch       = researchSourceQueries["TryLockResearch"]
	UnlockResearch        = researchSourceQueries["UnlockResearch"]
	NextResearchTopic     = researchSourceQueries["NextResearchTopic"]
	FindActiveRun         = researchSourceQueries["FindActiveRun"]
	StartResearch         = researchSourceQueries["StartResearch"]
	FailResearch          = researchSourceQueries["FailResearch"]
	SaveEvidence          = researchSourceQueries["SaveEvidence"]
	FinishResearch        = researchSourceQueries["FinishResearch"]
	QueueBebopMedia       = researchSourceQueries["QueueBebopMedia"]
	RecordResearchEvent   = researchSourceQueries["RecordResearchEvent"]
)
