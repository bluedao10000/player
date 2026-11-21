package knowledge

import _ "embed"

//go:embed editor_worker/editorial_episode_sources.sql
var EditorialEpisodeSources string

//go:embed editor_worker/research_documents.sql
var ResearchDocuments string

//go:embed editor_worker/claim_editor_task.sql
var ClaimEditorTask string

//go:embed editor_worker/check_editor_lease.sql
var CheckEditorLease string

//go:embed editor_worker/advance_editor_task.sql
var AdvanceEditorTask string
