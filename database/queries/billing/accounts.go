package billing

import _ "embed"

//go:embed accounts/create_rate.sql
var CreateRate string

//go:embed accounts/list_rates.sql
var ListRates string

//go:embed accounts/daily_statements.sql
var DailyStatements string

//go:embed accounts/daily_receipts.sql
var DailyReceipts string
