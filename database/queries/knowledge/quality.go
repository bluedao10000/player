package knowledge

import _ "embed"

//go:embed quality/lock_projection.sql
var LockProjection string

//go:embed quality/create_quality_scope.sql
var CreateQualityScope string

//go:embed quality/fill_quality_scope.sql
var FillQualityScope string

//go:embed quality/quality_report.sql
var QualityReport string
