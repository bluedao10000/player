package workbench

import _ "embed"
import resources "anime-agent/api/internal/repository/queries"

//go:embed media.queries
var mediaSource string

var (
	mediaSourceQueries = resources.Statements(mediaSource)
	MediaCandidates    = mediaSourceQueries["MediaCandidates"]
)
