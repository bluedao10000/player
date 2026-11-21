package metering

import _ "embed"

//go:embed ledger/save_model_call.sql
var SaveModelCall string

//go:embed ledger/match_existing_call.sql
var MatchExistingCall string
