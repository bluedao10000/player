package billing

import _ "embed"
import resources "anime-agent/api/internal/repository/queries"

//go:embed storekit.queries
var storekitSource string

var (
	storekitSourceQueries = resources.Statements(storekitSource)
	RecordStorePurchase   = storekitSourceQueries["RecordStorePurchase"]
)
