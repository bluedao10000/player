package knowledge

import (
	"context"
	_ "embed"

	"github.com/jackc/pgx/v5/pgxpool"

	"anime-agent/api/internal/repository/database"
	resources "anime-agent/api/internal/repository/queries"
)

//go:embed ops_completeness.queries
var opsCompletenessSource string

var opsCompletenessQueries = resources.Statements(opsCompletenessSource)

func OpsKnowledgeSummary(ctx context.Context, db *pgxpool.Pool, days int) ([]byte, error) {
	return database.ReadJSON(ctx, db, opsCompletenessQueries["OpsKnowledgeSummary"], days)
}

func OpsKnowledgeCompleteness(ctx context.Context, db *pgxpool.Pool, work string) ([]byte, error) {
	return database.ReadJSON(ctx, db, opsCompletenessQueries["OpsKnowledgeCompleteness"], work)
}

func OpsKnowledgeWorks(ctx context.Context, db *pgxpool.Pool, q, hasVersion, workType, completenessStatus, productionStatus, sort string, offset, limit int, sourceReviewOnly bool) ([]byte, error) {
	return database.ReadJSONWithoutJIT(ctx, db, opsCompletenessQueries["OpsKnowledgeWorks"], q, hasVersion, workType,
		completenessStatus, productionStatus, sort, offset, limit, sourceReviewOnly)
}

func OpsKnowledgeWork(ctx context.Context, db *pgxpool.Pool, work string) ([]byte, error) {
	return database.ReadJSON(ctx, db, opsCompletenessQueries["OpsKnowledgeWork"], work)
}
