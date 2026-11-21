package catalog

import _ "embed"

//go:embed works/works.sql
var Works string

//go:embed works/work.sql
var Work string

//go:embed works/research_runs.sql
var ResearchRuns string

//go:embed works/world_statuses.sql
var WorldStatuses string

//go:embed works/work_status.sql
var WorkStatus string

//go:embed works/sources.sql
var Sources string

//go:embed works/quality_runs.sql
var QualityRuns string

//go:embed works/sync_runs.sql
var SyncRuns string

//go:embed works/match_candidates.sql
var MatchCandidates string
