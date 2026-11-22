package knowledge

import _ "embed"
import resources "anime-agent/api/internal/repository/queries"

//go:embed plans.queries
var plansSource string

var (
	plansSourceQueries                    = resources.Statements(plansSource)
	RegisterArt                           = plansSourceQueries["RegisterArt"]
	ResolveEditionEpisodeCount            = plansSourceQueries["ResolveEditionEpisodeCount"]
	FindWorldVersion                      = plansSourceQueries["FindWorldVersion"]
	ResolveEditionWork                    = plansSourceQueries["ResolveEditionWork"]
	CreateEditionVersion                  = plansSourceQueries["CreateEditionVersion"]
	CreateWorldRelease                    = plansSourceQueries["CreateWorldRelease"]
	RegisterEditionWorld                  = plansSourceQueries["RegisterEditionWorld"]
	SeedRegisteredEditionMediaBatchWorlds = plansSourceQueries["SeedOpsMediaBatchWorlds"]
	AppendHomeSlot                        = plansSourceQueries["AppendHomeSlot"]
	RegisterTopic                         = plansSourceQueries["RegisterTopic"]
	RegisterPlan                          = plansSourceQueries["RegisterPlan"]
	RegisterExtension                     = plansSourceQueries["RegisterExtension"]
	HasPortrait                           = plansSourceQueries["HasPortrait"]
	LockPlanRegistration                  = plansSourceQueries["LockPlanRegistration"]
	HasManifest                           = plansSourceQueries["HasManifest"]
	SaveManifest                          = plansSourceQueries["SaveManifest"]
	EnqueueEpisodeArt                     = plansSourceQueries["EnqueueEpisodeArt"]
	ListDueAcceptances                    = plansSourceQueries["ListDueAcceptances"]
	TryLockEpisode                        = plansSourceQueries["TryLockEpisode"]
	AcceptanceState                       = plansSourceQueries["AcceptanceState"]
	BumpResearchPolicy                    = plansSourceQueries["BumpResearchPolicy"]
	SetAcceptedWorldCover                 = plansSourceQueries["SetAcceptedWorldCover"]
)
