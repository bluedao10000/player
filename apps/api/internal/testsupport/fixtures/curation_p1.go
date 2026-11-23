package fixtures

import _ "embed"
import resources "anime-agent/api/internal/repository/queries"

//go:embed curation_p1.queries
var curationP1Source string

var CurationP1 = resources.Statements(curationP1Source)
