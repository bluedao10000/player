package fixtures

import _ "embed"
import resources "anime-agent/api/internal/repository/queries"

//go:embed acceptance.queries
var acceptanceSource string

var (
	acceptanceSourceQueries    = resources.Statements(acceptanceSource)
	RenameTestWorld            = acceptanceSourceQueries["RenameTestWorld"]
	DeleteTestWorldOpsMappings = acceptanceSourceQueries["DeleteTestWorldOpsMappings"]
	CountAcceptances           = acceptanceSourceQueries["CountAcceptances"]
	CountPublishedScopes       = acceptanceSourceQueries["CountPublishedScopes"]
	UnpublishedChecks          = acceptanceSourceQueries["UnpublishedChecks"]
	LatestScopeReport          = acceptanceSourceQueries["LatestScopeReport"]
	CountEpisodeThreeFailures  = acceptanceSourceQueries["CountEpisodeThreeFailures"]
	OutstandingScopeChecks     = acceptanceSourceQueries["OutstandingScopeChecks"]
	CountPublishedExtensions   = acceptanceSourceQueries["CountPublishedExtensions"]
)
