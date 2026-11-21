package knowledge

import _ "embed"

//go:embed editor_sessions/create_editor_session.sql
var CreateEditorSession string

//go:embed editor_sessions/editor_session_operator.sql
var EditorSessionOperator string

//go:embed editor_sessions/delete_editor_session.sql
var DeleteEditorSession string
