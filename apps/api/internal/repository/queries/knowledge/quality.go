package knowledge

import _ "embed"
import resources "anime-agent/api/internal/repository/queries"

//go:embed quality.queries
var qualitySource string

var (
	qualitySourceQueries = resources.Statements(qualitySource)
	LockProjection       = qualitySourceQueries["LockProjection"]
	CreateQualityScope   = qualitySourceQueries["CreateQualityScope"]
	FillQualityScope     = qualitySourceQueries["FillQualityScope"]
	QualityReport        = qualitySourceQueries["QualityReport"]
)
