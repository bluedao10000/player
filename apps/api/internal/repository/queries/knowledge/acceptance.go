package knowledge

import _ "embed"
import resources "anime-agent/api/internal/repository/queries"

//go:embed acceptance.queries
var acceptanceSource string

var (
	acceptanceSourceQueries = resources.Statements(acceptanceSource)
	TryLockScopeAcceptance  = acceptanceSourceQueries["TryLockScopeAcceptance"]
	RecordScopeAcceptance   = acceptanceSourceQueries["RecordScopeAcceptance"]
	UpdateScopeHead         = acceptanceSourceQueries["UpdateScopeHead"]
	TryLockThirdEpisode     = acceptanceSourceQueries["TryLockThirdEpisode"]
	ThirdEpisodeState       = acceptanceSourceQueries["ThirdEpisodeState"]
)
