package fixtures

import _ "embed"

//go:embed schema/create_schema.sql
var CreateSchema string

//go:embed schema/drop_schema.sql
var DropSchema string
