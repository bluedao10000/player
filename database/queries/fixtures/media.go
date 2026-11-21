package fixtures

import _ "embed"

//go:embed media/set_review_stage.sql
var SetReviewStage string

//go:embed media/candidate_attempt.sql
var CandidateAttempt string

//go:embed media/insert_review_step.sql
var InsertReviewStep string

//go:embed media/candidate_focus.sql
var CandidateFocus string

//go:embed media/bebop_reference_candidate.sql
var BebopReferenceCandidate string

//go:embed media/insert_reference_step.sql
var InsertReferenceStep string

//go:embed media/candidate_reference.sql
var CandidateReference string

//go:embed media/mio_stage.sql
var MioStage string

//go:embed media/prepare_jet_portrait.sql
var PrepareJetPortrait string

//go:embed media/insert_portrait_render.sql
var InsertPortraitRender string

//go:embed media/count_jet_portraits.sql
var CountJetPortraits string

//go:embed media/published_portrait.sql
var PublishedPortrait string
