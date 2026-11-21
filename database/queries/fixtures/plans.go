package fixtures

import _ "embed"

//go:embed plans/create_naruto_edition.sql
var CreateNarutoEdition string

//go:embed plans/first_five_slugs.sql
var FirstFiveSlugs string

//go:embed plans/plan_graph_counts.sql
var PlanGraphCounts string

//go:embed plans/published_heads.sql
var PublishedHeads string

//go:embed plans/create_ona_edition.sql
var CreateONAEdition string

//go:embed plans/ona_coverage.sql
var ONACoverage string

//go:embed plans/fail_image_provider.sql
var FailImageProvider string

//go:embed plans/requeued_image.sql
var RequeuedImage string
