package knowledge

import _ "embed"
import resources "anime-agent/api/internal/repository/queries"

//go:embed editor_worker.queries
var editorworkerSource string

var (
	editorworkerSourceQueries = resources.Statements(editorworkerSource)
	EditorialEpisodeSources   = editorworkerSourceQueries["EditorialEpisodeSources"]
	ResearchDocuments         = editorworkerSourceQueries["ResearchDocuments"]
	ClaimEditorTask           = editorworkerSourceQueries["ClaimEditorTask"]
	CheckEditorLease          = editorworkerSourceQueries["CheckEditorLease"]
	AdvanceEditorTask         = editorworkerSourceQueries["AdvanceEditorTask"]
)
