package administration

import _ "embed"
import resources "anime-agent/api/internal/repository/queries"

//go:embed usage.queries
var usageSource string

var (
	usageSourceQueries = resources.Statements(usageSource)
	ModelUsage         = usageSourceQueries["ModelUsage"]
	Calls              = usageSourceQueries["Calls"]
)
