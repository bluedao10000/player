package billing

import _ "embed"
import resources "anime-agent/api/internal/repository/queries"

//go:embed accounts.queries
var accountsSource string

var (
	accountsSourceQueries = resources.Statements(accountsSource)
	CreateRate            = accountsSourceQueries["CreateRate"]
	ListRates             = accountsSourceQueries["ListRates"]
	DailyStatements       = accountsSourceQueries["DailyStatements"]
	DailyReceipts         = accountsSourceQueries["DailyReceipts"]
)
