package reader

import _ "embed"

//go:embed plans/lock_reader.sql
var LockReader string

//go:embed plans/set_reader_plan.sql
var SetReaderPlan string

//go:embed plans/record_plan_change.sql
var RecordPlanChange string
