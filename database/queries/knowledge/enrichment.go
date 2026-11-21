package knowledge

import _ "embed"

//go:embed enrichment/cached_world_page.sql
var CachedWorldPage string

//go:embed enrichment/set_source_backoff.sql
var SetSourceBackoff string

//go:embed enrichment/supplement_url.sql
var SupplementURL string
