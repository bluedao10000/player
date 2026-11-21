package knowledge

import _ "embed"

//go:embed media/finish_media_step.sql
var FinishMediaStep string

//go:embed media/publish_portrait.sql
var PublishPortrait string

//go:embed media/advance_candidate.sql
var AdvanceCandidate string

//go:embed media/requeue_provider_failures.sql
var RequeueProviderFailures string

//go:embed media/try_lock_media.sql
var TryLockMedia string

//go:embed media/unlock_media.sql
var UnlockMedia string

//go:embed media/next_candidate.sql
var NextCandidate string

//go:embed media/start_generation.sql
var StartGeneration string

//go:embed media/start_media_step.sql
var StartMediaStep string
