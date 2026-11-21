package knowledge

import _ "embed"
import resources "anime-agent/api/internal/repository/queries"

//go:embed enrichment.queries
var enrichmentSource string

var (
	enrichmentSourceQueries = resources.Statements(enrichmentSource)
	CachedWorldPage         = enrichmentSourceQueries["CachedWorldPage"]
	SetSourceBackoff        = enrichmentSourceQueries["SetSourceBackoff"]
	SupplementURL           = enrichmentSourceQueries["SupplementURL"]
)
