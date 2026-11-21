package reader

import _ "embed"
import resources "anime-agent/api/internal/repository/queries"

//go:embed plans.queries
var plansSource string

var (
	plansSourceQueries = resources.Statements(plansSource)
	LockReader         = plansSourceQueries["LockReader"]
	SetReaderPlan      = plansSourceQueries["SetReaderPlan"]
	RecordPlanChange   = plansSourceQueries["RecordPlanChange"]
)
