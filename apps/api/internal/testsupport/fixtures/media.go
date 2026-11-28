package fixtures

import _ "embed"
import resources "anime-agent/api/internal/repository/queries"

//go:embed media.queries
var mediaSource string

var (
	mediaSourceQueries      = resources.Statements(mediaSource)
	OnlyProviderTestSlots   = mediaSourceQueries["OnlyProviderTestSlots"]
	AgeProviderRetry        = mediaSourceQueries["AgeProviderRetry"]
	SetReviewStage          = mediaSourceQueries["SetReviewStage"]
	CandidateAttempt        = mediaSourceQueries["CandidateAttempt"]
	InsertReviewStep        = mediaSourceQueries["InsertReviewStep"]
	CandidateFocus          = mediaSourceQueries["CandidateFocus"]
	BebopReferenceCandidate = mediaSourceQueries["BebopReferenceCandidate"]
	InsertReferenceStep     = mediaSourceQueries["InsertReferenceStep"]
	CandidateReference      = mediaSourceQueries["CandidateReference"]
	MioStage                = mediaSourceQueries["MioStage"]
	PrepareJetPortrait      = mediaSourceQueries["PrepareJetPortrait"]
	InsertPortraitRender    = mediaSourceQueries["InsertPortraitRender"]
	CountJetPortraits       = mediaSourceQueries["CountJetPortraits"]
	PublishedPortrait       = mediaSourceQueries["PublishedPortrait"]
)
