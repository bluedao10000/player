package workbench

import _ "embed"
import resources "anime-agent/api/internal/repository/queries"

//go:embed editor.queries
var editorSource string

var (
	editorSourceQueries = resources.Statements(editorSource)
	Tasks               = editorSourceQueries["Tasks"]
	Task                = editorSourceQueries["Task"]
	Preview             = editorSourceQueries["Preview"]
)
