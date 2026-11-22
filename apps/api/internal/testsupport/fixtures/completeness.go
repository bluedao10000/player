package fixtures

import _ "embed"
import resources "anime-agent/api/internal/repository/queries"

//go:embed completeness.queries
var completenessSource string

var completenessQueries = resources.Statements(completenessSource)

var (
	SeedCompletenessExamples             = completenessQueries["SeedCompletenessExamples"]
	SeedCompletenessCatalogOnly          = completenessQueries["SeedCompletenessCatalogOnly"]
	SetCowboyType                        = completenessQueries["SetCowboyType"]
	SeedPrivateMediaCandidate            = completenessQueries["SeedPrivateMediaCandidate"]
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
	OpsKnowledgeExpectedSummary          = completenessQueries["OpsKnowledgeExpectedSummary"]
	CountCompletenessRuleForWorld        = completenessQueries["CountCompletenessRuleForWorld"]
	OpsLegacySnapshot                    = completenessQueries["OpsLegacySnapshot"]
	OpsSnapshotCompleteness              = completenessQueries["OpsSnapshotCompleteness"]
	OpsWorldSnapshotCompleteness         = completenessQueries["OpsWorldSnapshotCompleteness"]
	OpsLegacySnapshotState               = completenessQueries["OpsLegacySnapshotState"]
	OpsSnapshotActiveWorks               = completenessQueries["OpsSnapshotActiveWorks"]
)
