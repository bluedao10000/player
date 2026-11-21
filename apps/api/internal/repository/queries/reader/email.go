package reader

import _ "embed"
import resources "anime-agent/api/internal/repository/queries"

//go:embed email.queries
var emailSource string

var (
	emailSourceQueries        = resources.Statements(emailSource)
	LockCodeIssuer            = emailSourceQueries["LockCodeIssuer"]
	PurgeOldCodes             = emailSourceQueries["PurgeOldCodes"]
	CodeQuotaReached          = emailSourceQueries["CodeQuotaReached"]
	IssueCode                 = emailSourceQueries["IssueCode"]
	InvalidateUndeliveredCode = emailSourceQueries["InvalidateUndeliveredCode"]
	AttemptCode               = emailSourceQueries["AttemptCode"]
	ConsumeCode               = emailSourceQueries["ConsumeCode"]
	BindEmail                 = emailSourceQueries["BindEmail"]
	FindOrCreateReader        = emailSourceQueries["FindOrCreateReader"]
	PurgeSessions             = emailSourceQueries["PurgeSessions"]
	CreateSession             = emailSourceQueries["CreateSession"]
	AuthenticateSession       = emailSourceQueries["AuthenticateSession"]
	RevokeSession             = emailSourceQueries["RevokeSession"]
)
