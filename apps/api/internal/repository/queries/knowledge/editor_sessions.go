package knowledge

import _ "embed"
import resources "anime-agent/api/internal/repository/queries"

//go:embed editor_sessions.queries
var editorsessionsSource string

var (
	SetWorkbenchPassword           = editorsessionsSourceQueries["SetWorkbenchPassword"]
	RevokeWorkbenchSessions        = editorsessionsSourceQueries["RevokeWorkbenchSessions"]
	AttemptWorkbenchPassword       = editorsessionsSourceQueries["AttemptWorkbenchPassword"]
	ResetWorkbenchPasswordAttempts = editorsessionsSourceQueries["ResetWorkbenchPasswordAttempts"]

	editorsessionsSourceQueries = resources.Statements(editorsessionsSource)
	RegisterWorkbenchAccount    = editorsessionsSourceQueries["RegisterWorkbenchAccount"]
	CreateEditorSession         = editorsessionsSourceQueries["CreateEditorSession"]
	EditorSessionOperator       = editorsessionsSourceQueries["EditorSessionOperator"]
	DeleteEditorSession         = editorsessionsSourceQueries["DeleteEditorSession"]
)
