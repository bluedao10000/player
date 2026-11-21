package billing

import _ "embed"
import resources "anime-agent/api/internal/repository/queries"

//go:embed statements.queries
var statementsSource string

var (
	statementsSourceQueries  = resources.Statements(statementsSource)
	LockStatementBucket      = statementsSourceQueries["LockStatementBucket"]
	MatchStatement           = statementsSourceQueries["MatchStatement"]
	LatestStatementReference = statementsSourceQueries["LatestStatementReference"]
	InsertStatement          = statementsSourceQueries["InsertStatement"]
)
