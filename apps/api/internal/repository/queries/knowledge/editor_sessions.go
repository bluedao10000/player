package knowledge

import _ "embed"
import resources "anime-agent/api/internal/repository/queries"

//go:embed editor_sessions.queries
var editorsessionsSource string

var (
	editorsessionsSourceQueries = resources.Statements(editorsessionsSource)
	CreateEditorSession         = editorsessionsSourceQueries["CreateEditorSession"]
	EditorSessionOperator       = editorsessionsSourceQueries["EditorSessionOperator"]
	DeleteEditorSession         = editorsessionsSourceQueries["DeleteEditorSession"]
)
