package knowledge

import _ "embed"

//go:embed cli/list_evidence_due.sql
var ListEvidenceDue string

//go:embed cli/import_inventory.sql
var ImportInventory string

//go:embed cli/list_review_candidates.sql
var ListReviewCandidates string
