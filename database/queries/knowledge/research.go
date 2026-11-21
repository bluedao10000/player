package knowledge

import _ "embed"

//go:embed research/try_lock_research.sql
var TryLockResearch string

//go:embed research/unlock_research.sql
var UnlockResearch string

//go:embed research/next_research_topic.sql
var NextResearchTopic string

//go:embed research/find_active_run.sql
var FindActiveRun string

//go:embed research/start_research.sql
var StartResearch string

//go:embed research/fail_research.sql
var FailResearch string

//go:embed research/save_evidence.sql
var SaveEvidence string

//go:embed research/finish_research.sql
var FinishResearch string

//go:embed research/queue_bebop_media.sql
var QueueBebopMedia string

//go:embed research/record_research_event.sql
var RecordResearchEvent string
