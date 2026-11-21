package catalog

import _ "embed"
import resources "anime-agent/api/internal/repository/queries"

//go:embed works.queries
var worksSource string

var (
	worksSourceQueries = resources.Statements(worksSource)
	Works              = worksSourceQueries["Works"]
	Work               = worksSourceQueries["Work"]
	ResearchRuns       = worksSourceQueries["ResearchRuns"]
	WorldStatuses      = worksSourceQueries["WorldStatuses"]
	WorkStatus         = worksSourceQueries["WorkStatus"]
	Sources            = worksSourceQueries["Sources"]
	QualityRuns        = worksSourceQueries["QualityRuns"]
	SyncRuns           = worksSourceQueries["SyncRuns"]
	MatchCandidates    = worksSourceQueries["MatchCandidates"]
)
