package billing

import _ "embed"
import resources "anime-agent/api/internal/repository/queries"

//go:embed queries.queries
var queriesSource string

var (
	queriesSourceQueries = resources.Statements(queriesSource)
	Reconciliation       = queriesSourceQueries["Reconciliation"]
)
