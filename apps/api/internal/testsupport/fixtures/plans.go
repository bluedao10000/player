package fixtures

import _ "embed"
import resources "anime-agent/api/internal/repository/queries"

//go:embed plans.queries
var plansSource string

var (
	plansSourceQueries              = resources.Statements(plansSource)
	CreateNarutoEdition             = plansSourceQueries["CreateNarutoEdition"]
	SeedBlockedSourceBundleRequests = plansSourceQueries["SeedBlockedSourceBundleRequests"]
	AttachedSourceBundleCount       = plansSourceQueries["AttachedSourceBundleCount"]
	MoveSourceBundleToConfiguration = plansSourceQueries["MoveSourceBundleToConfiguration"]
	FirstFiveSlugs                  = plansSourceQueries["FirstFiveSlugs"]
	PlanGraphCounts                 = plansSourceQueries["PlanGraphCounts"]
	PublishedHeads                  = plansSourceQueries["PublishedHeads"]
	CreateONAEdition                = plansSourceQueries["CreateONAEdition"]
	ONACoverage                     = plansSourceQueries["ONACoverage"]
	FailImageProvider               = plansSourceQueries["FailImageProvider"]
	RequeuedImage                   = plansSourceQueries["RequeuedImage"]
)
