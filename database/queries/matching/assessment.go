package matching

import _ "embed"

//go:embed assessment/match_evidence.sql
var MatchEvidence string

//go:embed assessment/try_lock_assessment.sql
var TryLockAssessment string

//go:embed assessment/cached_assessment.sql
var CachedAssessment string

//go:embed assessment/save_assessment.sql
var SaveAssessment string
