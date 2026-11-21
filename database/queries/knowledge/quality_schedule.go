package knowledge

import _ "embed"

//go:embed quality_schedule/try_lock_quality.sql
var TryLockQuality string

//go:embed quality_schedule/fail_interrupted_quality.sql
var FailInterruptedQuality string

//go:embed quality_schedule/begin_quality_if_due.sql
var BeginQualityIfDue string

//go:embed quality_schedule/fail_quality.sql
var FailQuality string

//go:embed quality_schedule/finish_quality.sql
var FinishQuality string
