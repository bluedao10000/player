package knowledge

import _ "embed"
import resources "anime-agent/api/internal/repository/queries"

//go:embed source_review.queries
var sourceReviewSource string

var (
	sourceReviewQueries = resources.Statements(sourceReviewSource)
	SaveSourceReviews   = sourceReviewQueries["SaveSourceReviews"]
	ResolveSourceReview = sourceReviewQueries["ResolveSourceReview"]
)
