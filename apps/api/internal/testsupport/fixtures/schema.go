package fixtures

import _ "embed"
import resources "anime-agent/api/internal/repository/queries"

//go:embed schema.queries
var schemaSource string

var (
	schemaSourceQueries = resources.Statements(schemaSource)
	CreateSchema        = schemaSourceQueries["CreateSchema"]
	DropSchema          = schemaSourceQueries["DropSchema"]
)
