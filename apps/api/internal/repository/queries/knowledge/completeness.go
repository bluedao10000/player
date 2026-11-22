package knowledge

import _ "embed"
import resources "anime-agent/api/internal/repository/queries"

//go:embed completeness.queries
var completenessSource string

var completenessQueries = resources.Statements(completenessSource)

var (
	CalculateCompleteness = completenessQueries["CalculateCompleteness"]
	WorkCompleteness      = completenessQueries["WorkCompleteness"]
)
