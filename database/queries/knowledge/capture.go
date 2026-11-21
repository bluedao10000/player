package knowledge

import _ "embed"

//go:embed capture/finish_capture_lease.sql
var FinishCaptureLease string

//go:embed capture/save_capture.sql
var SaveCapture string
