package knowledge

import _ "embed"

//go:embed editor/record_editor_event.sql
var RecordEditorEvent string

//go:embed editor/lock_editor_task.sql
var LockEditorTask string

//go:embed editor/return_editor_task.sql
var ReturnEditorTask string

//go:embed editor/lock_editor_head.sql
var LockEditorHead string

//go:embed editor/compare_edit_base.sql
var CompareEditBase string

//go:embed editor/mark_edit_conflict.sql
var MarkEditConflict string

//go:embed editor/publish_editor_head.sql
var PublishEditorHead string

//go:embed editor/mark_edit_published.sql
var MarkEditPublished string

//go:embed editor/create_editor_task.sql
var CreateEditorTask string

//go:embed editor/find_duplicate_task.sql
var FindDuplicateTask string
