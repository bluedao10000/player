package fixtures

import _ "embed"
import resources "anime-agent/api/internal/repository/queries"

//go:embed completeness.queries
var completenessSource string

var completenessQueries = resources.Statements(completenessSource)

var (
	ReadCompletenessResults              = completenessQueries["ReadCompletenessResults"]
	SeedCompletenessExamples             = completenessQueries["SeedCompletenessExamples"]
	SeedCompletenessCatalogOnly          = completenessQueries["SeedCompletenessCatalogOnly"]
	SeedCompletenessP0Contracts          = completenessQueries["SeedCompletenessP0Contracts"]
	SetCowboyType                        = completenessQueries["SetCowboyType"]
	AddWhitespaceCompletenessLocation    = completenessQueries["AddWhitespaceCompletenessLocation"]
	RemoveWhitespaceCompletenessLocation = completenessQueries["RemoveWhitespaceCompletenessLocation"]
	DuplicateCompletenessLocation        = completenessQueries["DuplicateCompletenessLocation"]
	RestoreCompletenessLocation          = completenessQueries["RestoreCompletenessLocation"]
	SeedCompletenessPendingEvidence      = completenessQueries["SeedCompletenessPendingEvidence"]
	PublishCompletenessPendingEpisode    = completenessQueries["PublishCompletenessPendingEpisode"]
	CowboyPublishedVersion               = completenessQueries["CowboyPublishedVersion"]
	CreateCowboyCompletenessVersion      = completenessQueries["CreateCowboyCompletenessVersion"]
	CompletenessResultCountForVersion    = completenessQueries["CompletenessResultCountForVersion"]
	DeactivateCompletenessCatalogOnly    = completenessQueries["DeactivateCompletenessCatalogOnly"]
	CompletenessRuleCount                = completenessQueries["CompletenessRuleCount"]
	CompletenessResultCount              = completenessQueries["CompletenessResultCount"]

	OpsLegacySnapshot            = completenessQueries["OpsLegacySnapshot"]
	OpsSnapshotCompleteness      = completenessQueries["OpsSnapshotCompleteness"]
	OpsWorldSnapshotCompleteness = completenessQueries["OpsWorldSnapshotCompleteness"]
	OpsLegacySnapshotState       = completenessQueries["OpsLegacySnapshotState"]
	OpsSnapshotActiveWorks       = completenessQueries["OpsSnapshotActiveWorks"]
)
