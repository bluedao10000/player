package fixtures

import _ "embed"

//go:embed storekit/create_store_reader.sql
var CreateStoreReader string

//go:embed storekit/read_plan_credits.sql
var ReadPlanCredits string

//go:embed storekit/purchase_periods.sql
var PurchasePeriods string

//go:embed storekit/has_overlapping_periods.sql
var HasOverlappingPeriods string
