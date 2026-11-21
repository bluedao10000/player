package fixtures

import _ "embed"
import resources "anime-agent/api/internal/repository/queries"

//go:embed editor.queries
var editorSource string

var (
	editorSourceQueries    = resources.Statements(editorSource)
	CountActiveEditorTasks = editorSourceQueries["CountActiveEditorTasks"]
	EditorStatus           = editorSourceQueries["EditorStatus"]
	ProfileSnapshot        = editorSourceQueries["ProfileSnapshot"]
	CreateProfileReview    = editorSourceQueries["CreateProfileReview"]
	PublishedProfiles      = editorSourceQueries["PublishedProfiles"]
	PreviewWorldSetting    = editorSourceQueries["PreviewWorldSetting"]
	CountChapterHeads      = editorSourceQueries["CountChapterHeads"]
)
