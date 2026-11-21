package knowledge

import _ "embed"
import resources "anime-agent/api/internal/repository/queries"

//go:embed editor.queries
var editorSource string

var (
	editorSourceQueries = resources.Statements(editorSource)
	RecordEditorEvent   = editorSourceQueries["RecordEditorEvent"]
	LockEditorTask      = editorSourceQueries["LockEditorTask"]
	ReturnEditorTask    = editorSourceQueries["ReturnEditorTask"]
	LockEditorHead      = editorSourceQueries["LockEditorHead"]
	CompareEditBase     = editorSourceQueries["CompareEditBase"]
	MarkEditConflict    = editorSourceQueries["MarkEditConflict"]
	PublishEditorHead   = editorSourceQueries["PublishEditorHead"]
	MarkEditPublished   = editorSourceQueries["MarkEditPublished"]
	CreateEditorTask    = editorSourceQueries["CreateEditorTask"]
	FindDuplicateTask   = editorSourceQueries["FindDuplicateTask"]
)
