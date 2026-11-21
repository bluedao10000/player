package knowledge

import _ "embed"
import resources "anime-agent/api/internal/repository/queries"

//go:embed editor_sessions.queries
var editorsessionsSource string

var (
	editorsessionsSourceQueries = resources.Statements(editorsessionsSource)
	WorkbenchEditorGranted      = editorsessionsSourceQueries["WorkbenchEditorGranted"]
	GrantWorkbenchEditor        = editorsessionsSourceQueries["GrantWorkbenchEditor"]
	RevokeWorkbenchEditor       = editorsessionsSourceQueries["RevokeWorkbenchEditor"]
	RegisterWorkbenchAccount    = editorsessionsSourceQueries["RegisterWorkbenchAccount"]
	CreateEditorSession         = editorsessionsSourceQueries["CreateEditorSession"]
	EditorSessionOperator       = editorsessionsSourceQueries["EditorSessionOperator"]
	DeleteEditorSession         = editorsessionsSourceQueries["DeleteEditorSession"]
)
