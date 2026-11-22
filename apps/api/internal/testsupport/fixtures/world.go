package fixtures

import _ "embed"
import resources "anime-agent/api/internal/repository/queries"

//go:embed world.queries
var worldSource string

var (
	worldSourceQueries          = resources.Statements(worldSource)
	WorldRevision               = worldSourceQueries["WorldRevision"]
	InsertPreface               = worldSourceQueries["InsertPreface"]
	InsertSecondScene           = worldSourceQueries["InsertSecondScene"]
	InsertUnsupportedAppearance = worldSourceQueries["InsertUnsupportedAppearance"]
	CreateBebopWork             = worldSourceQueries["CreateBebopWork"]
	AttachBebopSource           = worldSourceQueries["AttachBebopSource"]
	SetBebopURLs                = worldSourceQueries["SetBebopURLs"]
	SetSupplementURL            = worldSourceQueries["SetSupplementURL"]
	PublishedCast               = worldSourceQueries["PublishedCast"]
	ArtCompleteness             = worldSourceQueries["ArtCompleteness"]
	TidepostVersion             = worldSourceQueries["TidepostVersion"]
	OpsWorldMappingCount        = worldSourceQueries["OpsWorldMappingCount"]
	TidepostOpsMappingCount     = worldSourceQueries["TidepostOpsMappingCount"]
	ForceWorldReimport          = worldSourceQueries["ForceWorldReimport"]
	RemoveSceneCast             = worldSourceQueries["RemoveSceneCast"]
	RemoveHomeSlot              = worldSourceQueries["RemoveHomeSlot"]
	RemoveWorld                 = worldSourceQueries["RemoveWorld"]
)
