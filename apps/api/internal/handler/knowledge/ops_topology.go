package knowledge

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	opsread "anime-agent/api/internal/repository/queries/ops"
)

type topologyCursor struct {
	At  string `json:"at"`
	Key string `json:"key"`
}

func topologyWindow(c *gin.Context) (string, int, bool) {
	window := c.DefaultQuery("window", "24h")
	hours := map[string]int{"24h": 24, "7d": 168, "30d": 720}
	value, ok := hours[window]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "window must be 24h, 7d, or 30d"})
		return "", 0, false
	}
	return window, value, true
}

func topologyWorld(c *gin.Context) (string, bool) {
	world := strings.TrimSpace(c.Query("world"))
	if len(world) > 512 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "world is too long"})
		return "", false
	}
	return world, true
}

func encodeTopologyCursor(at, key string) string {
	raw, _ := json.Marshal(topologyCursor{At: at, Key: key})
	return base64.RawURLEncoding.EncodeToString(raw)
}

func decodeTopologyCursor(raw string) (*string, string, bool) {
	if raw == "" {
		return nil, "", true
	}
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil, "", false
	}
	var cursor topologyCursor
	if json.Unmarshal(decoded, &cursor) != nil || cursor.Key == "" || cursor.At == "" {
		return nil, "", false
	}
	if cursor.At != "-infinity" {
		if _, err := time.Parse(time.RFC3339Nano, cursor.At); err != nil {
			return nil, "", false
		}
	}
	return &cursor.At, cursor.Key, true
}

func findTopologyStage(body []byte, id string) (json.RawMessage, bool) {
	var response struct {
		Groups []struct {
			Stages []json.RawMessage `json:"stages"`
		} `json:"groups"`
		Infrastructure struct {
			Stages []json.RawMessage `json:"stages"`
		} `json:"infrastructure"`
	}
	if json.Unmarshal(body, &response) != nil {
		return nil, false
	}
	for _, group := range response.Groups {
		for _, raw := range group.Stages {
			var stage struct {
				ID string `json:"id"`
			}
			if json.Unmarshal(raw, &stage) == nil && stage.ID == id {
				return raw, true
			}
		}
	}
	for _, raw := range response.Infrastructure.Stages {
		var stage struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(raw, &stage) == nil && stage.ID == id {
			return raw, true
		}
	}
	return nil, false
}

func topologyRoutes(g *gin.RouterGroup, db *pgxpool.Pool) {
	g.GET("/ops/topology", func(c *gin.Context) {
		window, hours, ok := topologyWindow(c)
		if !ok {
			return
		}
		world, ok := topologyWorld(c)
		if !ok {
			return
		}
		ctx, cancel := readContext(c)
		defer cancel()
		if world != "" {
			exists, err := opsread.TopologyWorldExists(ctx, db, world)
			if err != nil {
				writeJSON(c, nil, err)
				return
			}
			if !exists {
				c.JSON(http.StatusNotFound, gin.H{"error": "world not found"})
				return
			}
		}
		body, err := opsread.Topology(ctx, db, window, hours, world)
		writeJSON(c, body, err)
	})

	g.GET("/ops/topology/stages/:stage", func(c *gin.Context) {
		window, hours, ok := topologyWindow(c)
		if !ok {
			return
		}
		world, ok := topologyWorld(c)
		if !ok {
			return
		}
		stageID := strings.TrimSpace(c.Param("stage"))
		if stageID == "" || len(stageID) > 128 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid stage"})
			return
		}
		pageSize := limit(c)
		if pageSize == 0 {
			return
		}
		queueAt, queueKey, valid := decodeTopologyCursor(c.Query("queue_after"))
		if !valid {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid queue_after cursor"})
			return
		}
		runAt, runKey, valid := decodeTopologyCursor(c.Query("runs_after"))
		if !valid {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid runs_after cursor"})
			return
		}

		ctx, cancel := readContext(c)
		defer cancel()
		if world != "" {
			exists, err := opsread.TopologyWorldExists(ctx, db, world)
			if err != nil {
				writeJSON(c, nil, err)
				return
			}
			if !exists {
				c.JSON(http.StatusNotFound, gin.H{"error": "world not found"})
				return
			}
		}
		topology, err := opsread.Topology(ctx, db, window, hours, world)
		if err != nil {
			writeJSON(c, nil, err)
			return
		}
		stage, exists := findTopologyStage(topology, stageID)
		if !exists {
			c.JSON(http.StatusNotFound, gin.H{"error": "stage not found"})
			return
		}
		body, err := opsread.TopologyStageDetail(ctx, db, stageID, hours, world,
			queueAt, queueKey, runAt, runKey, pageSize+1)
		if err != nil {
			writeJSON(c, nil, err)
			return
		}
		var detail map[string]json.RawMessage
		if json.Unmarshal(body, &detail) != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "topology detail unavailable"})
			return
		}
		detail["stage"] = stage
		var queue []json.RawMessage
		var runs []json.RawMessage
		_ = json.Unmarshal(detail["current_queue"], &queue)
		_ = json.Unmarshal(detail["recent_runs"], &runs)
		queueNext, runsNext := "", ""
		if len(queue) > pageSize {
			queue = queue[:pageSize]
			var last struct {
				ItemKey string     `json:"item_key"`
				Since   *time.Time `json:"since"`
			}
			if json.Unmarshal(queue[len(queue)-1], &last) == nil && last.ItemKey != "" {
				at := "-infinity"
				if last.Since != nil {
					at = last.Since.UTC().Format(time.RFC3339Nano)
				}
				queueNext = encodeTopologyCursor(at, last.ItemKey)
			}
		}
		if len(runs) > pageSize {
			runs = runs[:pageSize]
			var last struct {
				RunKey     string     `json:"run_key"`
				StartedAt  *time.Time `json:"started_at"`
				FinishedAt *time.Time `json:"finished_at"`
			}
			if json.Unmarshal(runs[len(runs)-1], &last) == nil && last.RunKey != "" {
				at := last.StartedAt
				if last.FinishedAt != nil {
					at = last.FinishedAt
				}
				if at != nil {
					runsNext = encodeTopologyCursor(at.UTC().Format(time.RFC3339Nano), last.RunKey)
				}
			}
		}
		detail["current_queue"], _ = json.Marshal(queue)
		detail["recent_runs"], _ = json.Marshal(runs)
		pagination, _ := json.Marshal(map[string]any{
			"limit": pageSize, "queue_after": nullableCursor(queueNext), "runs_after": nullableCursor(runsNext),
		})
		detail["pagination"] = pagination
		result, marshalErr := json.Marshal(detail)
		writeJSON(c, result, marshalErr)
	})
}

func nullableCursor(value string) any {
	if value == "" {
		return nil
	}
	return value
}
