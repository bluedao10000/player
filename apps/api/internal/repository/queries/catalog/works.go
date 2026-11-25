package catalog

import _ "embed"
import resources "anime-agent/api/internal/repository/queries"

//go:embed works.queries
var worksSource string

var (
	worksSourceQueries = resources.Statements(worksSource)
	Works              = worksSourceQueries["Works"]
	Work               = worksSourceQueries["Work"]
	Sources            = worksSourceQueries["Sources"]
	MatchCandidates    = worksSourceQueries["MatchCandidates"]
)
