package workbench

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	catalog "anime-agent/api/internal/model/knowledge"
	"anime-agent/api/internal/repository/database"
	q "anime-agent/api/internal/repository/queries/workbench"
)

func (h handler) artifactResult(c *gin.Context) {
	kind, ref := c.Param("type"), c.Param("ref")
	if !oneOf(kind, "works-table", "coverage-matrix", "metric-comparison", "cost-breakdown") {
		apiFailure(c, invalid("type"))
		return
	}
	if ref == "" || len(ref) > 256 {
		apiFailure(c, invalid("ref"))
		return
	}
	if catalog.ValidUUID(ref) {
		payload, err := h.readJSON(c, "ArtifactResult", kind, ref, c.GetString("operator"))
		if !errors.Is(err, pgx.ErrNoRows) {
			respond(c, payload, err)
			return
		}
	}
	// Pre-upgrade SSE events used a date, run ID or an owned work-set ID.
	switch kind {
	case "works-table", "coverage-matrix":
		if catalog.ValidUUID(ref) {
			ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
			defer cancel()
			data, err := database.ReadJSON(ctx, h.db, q.Read["WorkSet"], ref, c.GetString("operator"))
			var ids []string
			if err == nil {
				err = json.Unmarshal(data, &ids)
			}
			if err != nil {
				apiFailure(c, err)
				return
			}
			h.read(c, "Works", "", "", "", "updated", 10, 0, ids)
			return
		}
	case "metric-comparison":
		start, err := time.ParseInLocation(time.DateOnly, ref, beijing)
		now := time.Now().In(beijing)
		if err == nil && !start.After(now) {
			to := start.AddDate(0, 0, 1)
			today := to.After(now)
			if today {
				to = now
			}
			body, err := h.readJSON(c, "Briefing", start, to, today)
			if err != nil {
				apiFailure(c, err)
				return
			}
			metrics, err := metricResult(body)
			if err != nil {
				apiFailure(c, err)
				return
			}
			c.JSON(200, metrics)
			return
		}
	case "cost-breakdown":
		h.read(c, "RunCostBreakdown", ref)
		return
	}
	apiFailure(c, pgx.ErrNoRows)
}

func metricResult(body []byte) (gin.H, error) {
	var out struct {
		Metrics json.RawMessage `json:"metrics"`
		AsOf    string          `json:"data_until"`
	}
	err := json.Unmarshal(body, &out)
	return gin.H{"metrics": out.Metrics, "as_of": out.AsOf}, err
}

func saveArtifactEvents(ctx context.Context, tx pgx.Tx, run string, input agentInput, requested []artifactRequest) ([]gin.H, error) {
	events := []gin.H{}
	seen := make(map[artifactRequest]bool)
	for _, artifact := range requested {
		var preferredID string
		var payload any
		switch artifact.Type {
		case "works-table", "coverage-matrix":
			if len(input.Works) == 0 {
				continue
			}
			artifact.Ref, preferredID = run, run
			rows := make([]json.RawMessage, 0, len(input.Works))
			for _, work := range input.Works {
				row := work.Work
				if artifact.Type == "coverage-matrix" {
					var fields map[string]json.RawMessage
					if err := json.Unmarshal(row, &fields); err != nil {
						return nil, err
					}
					fields["dimensions"] = work.Dimensions
					var err error
					row, err = json.Marshal(fields)
					if err != nil {
						return nil, err
					}
				}
				rows = append(rows, row)
			}
			payload = gin.H{"rows": rows, "total": len(rows)}
		case "metric-comparison":
			if len(input.briefing) == 0 {
				continue
			}
			artifact.Ref = input.BriefingDay
			var err error
			payload, err = metricResult(input.briefing)
			if err != nil {
				return nil, err
			}
		case "cost-breakdown":
			if !input.runRefs[artifact.Ref] {
				continue
			}
			body, err := database.ReadJSON(ctx, tx, q.Read["RunCostBreakdown"], artifact.Ref)
			if err != nil {
				return nil, err
			}
			payload = json.RawMessage(body)
		default:
			continue
		}
		if seen[artifact] {
			continue
		}
		seen[artifact] = true
		var ref string
		if err := tx.QueryRow(ctx, q.Write["SaveArtifactResult"], preferredID, artifact.Type, run, payload).Scan(&ref); err != nil {
			return nil, err
		}
		events = append(events, gin.H{"type": "artifact", "artifact": artifactRequest{Type: artifact.Type, Ref: ref}})
	}
	return events, nil
}
