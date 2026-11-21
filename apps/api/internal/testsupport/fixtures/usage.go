package fixtures

import _ "embed"
import resources "anime-agent/api/internal/repository/queries"

//go:embed usage.queries
var usageSource string

var (
	usageSourceQueries  = resources.Statements(usageSource)
	ReceiptTotals       = usageSourceQueries["ReceiptTotals"]
	SanitizedUsage      = usageSourceQueries["SanitizedUsage"]
	ConflictingReceipts = usageSourceQueries["ConflictingReceipts"]
	PartialReceipts     = usageSourceQueries["PartialReceipts"]
	OfflineUsageTotals  = usageSourceQueries["OfflineUsageTotals"]
)
