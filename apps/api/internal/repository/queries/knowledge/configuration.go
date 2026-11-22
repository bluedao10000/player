package knowledge

import (
	resources "anime-agent/api/internal/repository/queries"
	_ "embed"
)

//go:embed configuration.queries
var configurationSource string

var Configuration = resources.Statements(configurationSource)
