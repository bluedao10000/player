package fixtures

import (
	_ "embed"

	"anime-agent/api/internal/repository/queries"
)

//go:embed chat_tools.queries
var chatToolSource string

var ChatToolFixtures = queries.Statements(chatToolSource)
