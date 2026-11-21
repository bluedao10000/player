package workbench

import _ "embed"

//go:embed editor/tasks.sql
var Tasks string

//go:embed editor/task.sql
var Task string

//go:embed editor/preview.sql
var Preview string
