package knowledge

import (
	resources "anime-agent/api/internal/repository/queries"
	_ "embed"
)

//go:embed chat.queries
var chatSource string

var ChatQueries = resources.Statements(chatSource)
