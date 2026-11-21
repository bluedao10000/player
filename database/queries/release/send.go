package release

import _ "embed"

//go:embed send/try_lock_sender.sql
var TryLockSender string

//go:embed send/interrupt_open_attempts.sql
var InterruptOpenAttempts string

//go:embed send/claim_release.sql
var ClaimRelease string

//go:embed send/record_delivery.sql
var RecordDelivery string
