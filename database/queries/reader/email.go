package reader

import _ "embed"

//go:embed email/lock_code_issuer.sql
var LockCodeIssuer string

//go:embed email/purge_old_codes.sql
var PurgeOldCodes string

//go:embed email/code_quota_reached.sql
var CodeQuotaReached string

//go:embed email/issue_code.sql
var IssueCode string

//go:embed email/invalidate_undelivered_code.sql
var InvalidateUndeliveredCode string

//go:embed email/attempt_code.sql
var AttemptCode string

//go:embed email/consume_code.sql
var ConsumeCode string

//go:embed email/bind_email.sql
var BindEmail string

//go:embed email/find_or_create_reader.sql
var FindOrCreateReader string

//go:embed email/purge_sessions.sql
var PurgeSessions string

//go:embed email/create_session.sql
var CreateSession string

//go:embed email/authenticate_session.sql
var AuthenticateSession string

//go:embed email/revoke_session.sql
var RevokeSession string
