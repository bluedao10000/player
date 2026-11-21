package fixtures

import _ "embed"

//go:embed schedule/create_schedule_edition.sql
var CreateScheduleEdition string

//go:embed schedule/count_admissions.sql
var CountAdmissions string

//go:embed schedule/insert_missing_episodes.sql
var InsertMissingEpisodes string

//go:embed schedule/queue_status.sql
var QueueStatus string
