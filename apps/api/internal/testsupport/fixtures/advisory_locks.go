package fixtures

import (
	_ "embed"

	"anime-agent/api/internal/repository/queries"
)

//go:embed advisory_locks.queries
var lockSource string

var AdvisoryLocks = queries.Statements(lockSource)
