package metering

import _ "embed"
import resources "anime-agent/api/internal/repository/queries"

//go:embed ledger.queries
var ledgerSource string

var (
	ledgerSourceQueries = resources.Statements(ledgerSource)
	SaveModelCall       = ledgerSourceQueries["SaveModelCall"]
	MatchExistingCall   = ledgerSourceQueries["MatchExistingCall"]
)
