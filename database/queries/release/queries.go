package release

import _ "embed"

//go:embed export.sql
var ExportRelease string

//go:embed validate.sql
var ValidateRelease string

//go:embed import.sql
var ImportRelease string
