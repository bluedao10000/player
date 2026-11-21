package knowledge

import _ "embed"

//go:embed history/archived_version.sql
var ArchivedVersion string

//go:embed history/mark_source_works.sql
var MarkSourceWorks string
