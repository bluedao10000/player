package fixtures

import _ "embed"

//go:embed editor/count_active_editor_tasks.sql
var CountActiveEditorTasks string

//go:embed editor/editor_status.sql
var EditorStatus string

//go:embed editor/profile_snapshot.sql
var ProfileSnapshot string

//go:embed editor/create_profile_review.sql
var CreateProfileReview string

//go:embed editor/published_profiles.sql
var PublishedProfiles string

//go:embed editor/preview_world_setting.sql
var PreviewWorldSetting string

//go:embed editor/count_chapter_heads.sql
var CountChapterHeads string
