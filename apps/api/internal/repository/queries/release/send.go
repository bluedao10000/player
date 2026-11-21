package release

import _ "embed"
import resources "anime-agent/api/internal/repository/queries"

//go:embed send.queries
var sendSource string

var (
	sendSourceQueries     = resources.Statements(sendSource)
	TryLockSender         = sendSourceQueries["TryLockSender"]
	InterruptOpenAttempts = sendSourceQueries["InterruptOpenAttempts"]
	ClaimRelease          = sendSourceQueries["ClaimRelease"]
	RecordDelivery        = sendSourceQueries["RecordDelivery"]
)
