package release

import _ "embed"
import resources "anime-agent/api/internal/repository/queries"

//go:embed receive.queries
var receiveSource string

var (
	receiveSourceQueries = resources.Statements(receiveSource)
	LockPublication      = receiveSourceQueries["LockPublication"]
	FindReceipt          = receiveSourceQueries["FindReceipt"]
	HasChapterConflict   = receiveSourceQueries["HasChapterConflict"]
	InsertReceipt        = receiveSourceQueries["InsertReceipt"]
	VerifyInstalledScope = receiveSourceQueries["VerifyInstalledScope"]
)
