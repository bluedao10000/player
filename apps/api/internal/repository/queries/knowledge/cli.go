package knowledge

import _ "embed"
import resources "anime-agent/api/internal/repository/queries"

//go:embed cli.queries
var cliSource string

var (
	cliSourceQueries     = resources.Statements(cliSource)
	ListEvidenceDue      = cliSourceQueries["ListEvidenceDue"]
	ImportInventory      = cliSourceQueries["ImportInventory"]
	ListReviewCandidates = cliSourceQueries["ListReviewCandidates"]
)
