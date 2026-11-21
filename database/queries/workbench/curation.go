package workbench

import _ "embed"

//go:embed curation/versions.sql
var Versions string

//go:embed curation/matches.sql
var Matches string

//go:embed curation/match.sql
var Match string

//go:embed curation/issues.sql
var Issues string

//go:embed curation/history.sql
var History string

//go:embed curation/assessments.sql
var Assessments string
