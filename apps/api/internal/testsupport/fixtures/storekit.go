package fixtures

import _ "embed"
import resources "anime-agent/api/internal/repository/queries"

//go:embed storekit.queries
var storekitSource string

var (
	storekitSourceQueries = resources.Statements(storekitSource)
	CreateStoreReader     = storekitSourceQueries["CreateStoreReader"]
	ReadPlanCredits       = storekitSourceQueries["ReadPlanCredits"]
	PurchasePeriods       = storekitSourceQueries["PurchasePeriods"]
	HasOverlappingPeriods = storekitSourceQueries["HasOverlappingPeriods"]
)
