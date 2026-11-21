package catalog

import _ "embed"
import resources "anime-agent/api/internal/repository/queries"

//go:embed worlds.queries
var worldsSource string

var (
	worldsSourceQueries = resources.Statements(worldsSource)
	ScopeAcceptances    = worldsSourceQueries["ScopeAcceptances"]
	Discovery           = worldsSourceQueries["Discovery"]
	HomeWorlds          = worldsSourceQueries["HomeWorlds"]
	World               = worldsSourceQueries["World"]
	Character           = worldsSourceQueries["Character"]
	Chapter             = worldsSourceQueries["Chapter"]
	Scene               = worldsSourceQueries["Scene"]
)
