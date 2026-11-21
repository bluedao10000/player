package fixtures

import _ "embed"

//go:embed usage/receipt_totals.sql
var ReceiptTotals string

//go:embed usage/sanitized_usage.sql
var SanitizedUsage string

//go:embed usage/conflicting_receipts.sql
var ConflictingReceipts string

//go:embed usage/partial_receipts.sql
var PartialReceipts string

//go:embed usage/offline_usage_totals.sql
var OfflineUsageTotals string
