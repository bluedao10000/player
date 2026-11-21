package fixtures

import _ "embed"
import resources "anime-agent/api/internal/repository/queries"

//go:embed alipay.queries
var alipaySource string

var (
	alipaySourceQueries     = resources.Statements(alipaySource)
	CreateMembershipReader  = alipaySourceQueries["CreateMembershipReader"]
	CreateMembershipSession = alipaySourceQueries["CreateMembershipSession"]
	CountPaidTerms          = alipaySourceQueries["CountPaidTerms"]
)
