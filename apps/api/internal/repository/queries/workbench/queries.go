package workbench

import _ "embed"
import resources "anime-agent/api/internal/repository/queries"

//go:embed queries.queries
var queriesSource string

var (
	queriesSourceQueries = resources.Statements(queriesSource)
	ProductionUsage      = queriesSourceQueries["ProductionUsage"]
)
