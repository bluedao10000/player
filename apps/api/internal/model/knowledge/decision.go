package knowledge

import (
	"encoding/json"
	"regexp"

	"github.com/jackc/pgx/v5/pgtype"
)

type Decision struct {
	Action    string          `json:"action"`
	Reason    string          `json:"reason"`
	Work      string          `json:"work_id"`
	Field     string          `json:"field"`
	Value     json.RawMessage `json:"value"`
	Candidate int64           `json:"candidate_id"`
	Record    int64           `json:"record_id"`
}

func ValidUUID(s string) bool { var id pgtype.UUID; return len(s) == 36 && id.Scan(s) == nil }

var PortraitFilename = regexp.MustCompile(`^[a-f0-9]{64}\.webp$`)
