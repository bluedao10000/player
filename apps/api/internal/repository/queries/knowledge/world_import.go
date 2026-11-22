package knowledge

import _ "embed"
import resources "anime-agent/api/internal/repository/queries"

//go:embed world_import.queries
var worldimportSource string

var (
	worldimportSourceQueries          = resources.Statements(worldimportSource)
	CacheWorldPage                    = worldimportSourceQueries["CacheWorldPage"]
	SourceInBackoff                   = worldimportSourceQueries["SourceInBackoff"]
	SkipRateLimitedWorld              = worldimportSourceQueries["SkipRateLimitedWorld"]
	WorldManifest                     = worldimportSourceQueries["WorldManifest"]
	SkipUnchangedWorld                = worldimportSourceQueries["SkipUnchangedWorld"]
	StageWorld                        = worldimportSourceQueries["StageWorld"]
	RegisterWorldVersion              = worldimportSourceQueries["RegisterWorldVersion"]
	PublishWorldRelease               = worldimportSourceQueries["PublishWorldRelease"]
	SaveWorld                         = worldimportSourceQueries["SaveWorld"]
	SeedImportedWorldMediaBatchWorlds = worldimportSourceQueries["SeedOpsMediaBatchWorlds"]
	SetHomeSlot                       = worldimportSourceQueries["SetHomeSlot"]
	AdvanceWorldVersion               = worldimportSourceQueries["AdvanceWorldVersion"]
	FinishWorldImport                 = worldimportSourceQueries["FinishWorldImport"]
	FindTVEdition                     = worldimportSourceQueries["FindTVEdition"]
	LinkedWorldWork                   = worldimportSourceQueries["LinkedWorldWork"]
	CreateOriginalWork                = worldimportSourceQueries["CreateOriginalWork"]
	UpsertWorldRecord                 = worldimportSourceQueries["UpsertWorldRecord"]
	CreateAffectedWorks               = worldimportSourceQueries["CreateAffectedWorks"]
	InsertEvidence                    = worldimportSourceQueries["InsertEvidence"]
	InsertCharacter                   = worldimportSourceQueries["InsertCharacter"]
	InsertChapter                     = worldimportSourceQueries["InsertChapter"]
	InsertBlock                       = worldimportSourceQueries["InsertBlock"]
	InsertAppearance                  = worldimportSourceQueries["InsertAppearance"]
	InsertSetting                     = worldimportSourceQueries["InsertSetting"]
	InsertScene                       = worldimportSourceQueries["InsertScene"]
	InsertSceneAppearance             = worldimportSourceQueries["InsertSceneAppearance"]
	InsertStoryline                   = worldimportSourceQueries["InsertStoryline"]
	InsertStorylineChapter            = worldimportSourceQueries["InsertStorylineChapter"]
)
