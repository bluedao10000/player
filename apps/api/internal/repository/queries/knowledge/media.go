package knowledge

import _ "embed"
import resources "anime-agent/api/internal/repository/queries"

//go:embed media.queries
var mediaSource string

var (
	mediaSourceQueries      = resources.Statements(mediaSource)
	FinishMediaStep         = mediaSourceQueries["FinishMediaStep"]
	PublishPortrait         = mediaSourceQueries["PublishPortrait"]
	AdvanceCandidate        = mediaSourceQueries["AdvanceCandidate"]
	RequeueProviderFailures = mediaSourceQueries["RequeueProviderFailures"]
	ResumeCorrectedImage    = mediaSourceQueries["ResumeCorrectedImage"]
	TryLockMedia            = mediaSourceQueries["TryLockMedia"]
	UnlockMedia             = mediaSourceQueries["UnlockMedia"]
	NextCandidate           = mediaSourceQueries["NextCandidate"]
	StartGeneration         = mediaSourceQueries["StartGeneration"]
	StartMediaStep          = mediaSourceQueries["StartMediaStep"]
)
