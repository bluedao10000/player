package knowledge

import _ "embed"

//go:embed acceptance/try_lock_scope_acceptance.sql
var TryLockScopeAcceptance string

//go:embed acceptance/record_scope_acceptance.sql
var RecordScopeAcceptance string

//go:embed acceptance/update_scope_head.sql
var UpdateScopeHead string

//go:embed acceptance/try_lock_third_episode.sql
var TryLockThirdEpisode string

//go:embed acceptance/third_episode_state.sql
var ThirdEpisodeState string
