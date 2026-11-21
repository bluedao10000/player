package billing

import _ "embed"

//go:embed membership/plan_summary.sql
var PlanSummary string

//go:embed membership/lock_reader.sql
var LockReader string

//go:embed membership/is_paid_member.sql
var IsPaidMember string

//go:embed membership/pending_plan_order.sql
var PendingPlanOrder string

//go:embed membership/create_plan_order.sql
var CreatePlanOrder string

//go:embed membership/lock_plan_order_reader.sql
var LockPlanOrderReader string

//go:embed membership/lock_plan_payment.sql
var LockPlanPayment string

//go:embed membership/refund_plan_order.sql
var RefundPlanOrder string

//go:embed membership/close_plan_order.sql
var ClosePlanOrder string

//go:embed membership/activate_paid_period.sql
var ActivatePaidPeriod string
