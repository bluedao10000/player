package knowledge

import _ "embed"

//go:embed schedule/match_edition.sql
var MatchEdition string

//go:embed schedule/save_policy.sql
var SavePolicy string

//go:embed schedule/pause_unadmitted_topics.sql
var PauseUnadmittedTopics string

//go:embed schedule/admit_episode.sql
var AdmitEpisode string

//go:embed schedule/lock_capture_queue.sql
var LockCaptureQueue string

//go:embed schedule/expire_capture_leases.sql
var ExpireCaptureLeases string

//go:embed schedule/claim_capture_lease.sql
var ClaimCaptureLease string
