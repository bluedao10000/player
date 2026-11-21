package knowledge

import _ "embed"
import resources "anime-agent/api/internal/repository/queries"

//go:embed archive.queries
var archiveSource string

var (
	archiveSourceQueries = resources.Statements(archiveSource)
	FindEnabledSource    = archiveSourceQueries["FindEnabledSource"]
)
