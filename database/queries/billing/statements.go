package billing

import _ "embed"

//go:embed statements/lock_statement_bucket.sql
var LockStatementBucket string

//go:embed statements/match_statement.sql
var MatchStatement string

//go:embed statements/latest_statement_reference.sql
var LatestStatementReference string

//go:embed statements/insert_statement.sql
var InsertStatement string
