// Package workbench embeds the read projections and transactions of the workbench API.
package workbench

import (
	_ "embed"

	"anime-agent/api/internal/repository/queries"
)

//go:embed reads.queries
var reads string

//go:embed writes.queries
var writes string

var Read = readStatements()

func readStatements() map[string]string {
	statements := queries.Statements(reads)
	// Keep the shared run projection in the API query, so upgrades can replace
	// the underlying ops views without a dependent workbench view.
	for _, name := range []string{"Runs", "Run", "AgentRun", "RunExists", "Briefing", "RunsStats", "RunCostBreakdown"} {
		statements[name] = statements["RunProjection"] + statements[name]
	}
	delete(statements, "RunProjection")
	return statements
}

var Write = queries.Statements(writes)
