package fixtures

import _ "embed"
import resources "anime-agent/api/internal/repository/queries"

//go:embed auth.queries
var authSource string

var (
	authSourceQueries = resources.Statements(authSource)
	ReaderByUnionID   = authSourceQueries["ReaderByUnionID"]
	SeedEmailCode     = authSourceQueries["SeedEmailCode"]
	CountPlanChanges  = authSourceQueries["CountPlanChanges"]
	ExpireMembership  = authSourceQueries["ExpireMembership"]
	ResetEmailCode    = authSourceQueries["ResetEmailCode"]
)
