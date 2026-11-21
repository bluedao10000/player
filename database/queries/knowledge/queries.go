package knowledge

import _ "embed"

//go:embed merge.sql
var Merge string

//go:embed projection.sql
var Projection string

//go:embed quality.sql
var Quality string

//go:embed acceptance_inputs.sql
var AcceptanceInputs string
