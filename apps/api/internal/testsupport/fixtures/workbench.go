package fixtures

import (
	_ "embed"

	"anime-agent/api/internal/repository/queries"
)

//go:embed workbench.queries
var workbenchSource string

var Workbench = queries.Statements(workbenchSource)
