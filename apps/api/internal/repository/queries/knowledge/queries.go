package knowledge

import _ "embed"
import resources "anime-agent/api/internal/repository/queries"

//go:embed queries.queries
var queriesSource string

var (
	queriesSourceQueries = resources.Statements(queriesSource)
	Merge                = queriesSourceQueries["Merge"]
	Projection           = queriesSourceQueries["Projection"]
	Quality              = queriesSourceQueries["Quality"]
	AcceptanceInputs     = queriesSourceQueries["AcceptanceInputs"]
)
