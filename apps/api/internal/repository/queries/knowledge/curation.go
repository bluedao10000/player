package knowledge

import _ "embed"
import resources "anime-agent/api/internal/repository/queries"

//go:embed curation.queries
var curationSource string

var (
	curationSourceQueries = resources.Statements(curationSource)
	BeginCuration         = curationSourceQueries["BeginCuration"]
	ReadActiveField       = curationSourceQueries["ReadActiveField"]
	DeleteOverride        = curationSourceQueries["DeleteOverride"]
	UpsertOverride        = curationSourceQueries["UpsertOverride"]
	MarkAffectedWork      = curationSourceQueries["MarkAffectedWork"]
	ActiveRecordWork      = curationSourceQueries["ActiveRecordWork"]
	MatchPair             = curationSourceQueries["MatchPair"]
	LastCurationSource    = curationSourceQueries["LastCurationSource"]
	CloneWork             = curationSourceQueries["CloneWork"]
	MarkAffectedWorks     = curationSourceQueries["MarkAffectedWorks"]
	FindDecisionRecords   = curationSourceQueries["FindDecisionRecords"]
	RejectMatch           = curationSourceQueries["RejectMatch"]
	DeleteMatch           = curationSourceQueries["DeleteMatch"]
	DeleteIdentityPin     = curationSourceQueries["DeleteIdentityPin"]
	ClearRejections       = curationSourceQueries["ClearRejections"]
	PinRecords            = curationSourceQueries["PinRecords"]
	MoveRecords           = curationSourceQueries["MoveRecords"]
	ReadWorkField         = curationSourceQueries["ReadWorkField"]
	RecordDecision        = curationSourceQueries["RecordDecision"]
)
