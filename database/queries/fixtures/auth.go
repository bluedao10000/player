package fixtures

import _ "embed"

//go:embed auth/reader_by_union_id.sql
var ReaderByUnionID string

//go:embed auth/seed_email_code.sql
var SeedEmailCode string

//go:embed auth/count_plan_changes.sql
var CountPlanChanges string

//go:embed auth/expire_membership.sql
var ExpireMembership string

//go:embed auth/reset_email_code.sql
var ResetEmailCode string
