package fixtures

import _ "embed"

//go:embed acceptance/rename_test_world.sql
var RenameTestWorld string

//go:embed acceptance/count_acceptances.sql
var CountAcceptances string

//go:embed acceptance/count_published_scopes.sql
var CountPublishedScopes string

//go:embed acceptance/unpublished_checks.sql
var UnpublishedChecks string

//go:embed acceptance/latest_scope_report.sql
var LatestScopeReport string

//go:embed acceptance/count_episode_three_failures.sql
var CountEpisodeThreeFailures string

//go:embed acceptance/outstanding_scope_checks.sql
var OutstandingScopeChecks string

//go:embed acceptance/count_published_extensions.sql
var CountPublishedExtensions string
