package catalog

import _ "embed"
import resources "anime-agent/api/internal/repository/queries"

//go:embed media.queries
var mediaSource string

var (
	mediaSourceQueries  = resources.Statements(mediaSource)
	IsPublishedPortrait = mediaSourceQueries["IsPublishedPortrait"]
	IsPublishedScene    = mediaSourceQueries["IsPublishedScene"]
)
