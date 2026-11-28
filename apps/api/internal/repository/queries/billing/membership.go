package billing

import _ "embed"
import resources "anime-agent/api/internal/repository/queries"

//go:embed membership.queries
var membershipSource string

var (
	membershipSourceQueries = resources.Statements(membershipSource)
	PlanSummary             = membershipSourceQueries["PlanSummary"]
	IsPaidMember            = membershipSourceQueries["IsPaidMember"]
	PendingPlanOrder        = membershipSourceQueries["PendingPlanOrder"]
	CreatePlanOrder         = membershipSourceQueries["CreatePlanOrder"]
	LockPlanOrderReader     = membershipSourceQueries["LockPlanOrderReader"]
	LockPlanPayment         = membershipSourceQueries["LockPlanPayment"]
	RefundPlanOrder         = membershipSourceQueries["RefundPlanOrder"]
	ClosePlanOrder          = membershipSourceQueries["ClosePlanOrder"]
	ActivatePaidPeriod      = membershipSourceQueries["ActivatePaidPeriod"]
)
