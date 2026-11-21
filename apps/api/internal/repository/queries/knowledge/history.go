package knowledge

import _ "embed"
import resources "anime-agent/api/internal/repository/queries"

//go:embed history.queries
var historySource string

var (
	historySourceQueries = resources.Statements(historySource)
	ArchivedVersion      = historySourceQueries["ArchivedVersion"]
	MarkSourceWorks      = historySourceQueries["MarkSourceWorks"]
)
