package billing

import _ "embed"

//go:embed storekit/record_store_purchase.sql
var RecordStorePurchase string
