package knowledge

import _ "embed"
import resources "anime-agent/api/internal/repository/queries"

//go:embed capture.queries
var captureSource string

var (
	captureSourceQueries = resources.Statements(captureSource)
	FinishCaptureLease   = captureSourceQueries["FinishCaptureLease"]
	SaveCapture          = captureSourceQueries["SaveCapture"]
)
