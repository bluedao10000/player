package administration

import _ "embed"
import resources "anime-agent/api/internal/repository/queries"

//go:embed curation.queries
var curationSource string

var (
	curationSourceQueries = resources.Statements(curationSource)
	Versions              = curationSourceQueries["Versions"]
	Matches               = curationSourceQueries["Matches"]
	Match                 = curationSourceQueries["Match"]
	Issues                = curationSourceQueries["Issues"]
	History               = curationSourceQueries["History"]
	Assessments           = curationSourceQueries["Assessments"]
)
