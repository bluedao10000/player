package knowledge

import _ "embed"

//go:embed plans/register_art.sql
var RegisterArt string

//go:embed plans/resolve_edition_episode_count.sql
var ResolveEditionEpisodeCount string

//go:embed plans/find_world_version.sql
var FindWorldVersion string

//go:embed plans/resolve_edition_work.sql
var ResolveEditionWork string

//go:embed plans/create_edition_version.sql
var CreateEditionVersion string

//go:embed plans/create_world_release.sql
var CreateWorldRelease string

//go:embed plans/register_edition_world.sql
var RegisterEditionWorld string

//go:embed plans/append_home_slot.sql
var AppendHomeSlot string

//go:embed plans/register_topic.sql
var RegisterTopic string

//go:embed plans/register_plan.sql
var RegisterPlan string

//go:embed plans/register_extension.sql
var RegisterExtension string

//go:embed plans/has_portrait.sql
var HasPortrait string

//go:embed plans/lock_plan_registration.sql
var LockPlanRegistration string

//go:embed plans/has_manifest.sql
var HasManifest string

//go:embed plans/save_manifest.sql
var SaveManifest string

//go:embed plans/enqueue_episode_art.sql
var EnqueueEpisodeArt string

//go:embed plans/list_due_acceptances.sql
var ListDueAcceptances string

//go:embed plans/try_lock_episode.sql
var TryLockEpisode string

//go:embed plans/acceptance_state.sql
var AcceptanceState string

//go:embed plans/bump_research_policy.sql
var BumpResearchPolicy string

//go:embed plans/set_accepted_world_cover.sql
var SetAcceptedWorldCover string
