package workbench

import _ "embed"
import resources "anime-agent/api/internal/repository/queries"

//go:embed usage.queries
var usageSource string

var (
	usageSourceQueries = resources.Statements(usageSource)
	BackupStatus       = usageSourceQueries["BackupStatus"]
	EpisodeQueue       = usageSourceQueries["EpisodeQueue"]
	ReaderReleases     = usageSourceQueries["ReaderReleases"]
	EpisodeInventory   = usageSourceQueries["EpisodeInventory"]
	ModelUsage         = usageSourceQueries["ModelUsage"]
	Calls              = usageSourceQueries["Calls"]
)
