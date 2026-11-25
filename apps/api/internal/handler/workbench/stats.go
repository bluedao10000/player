package workbench

import (
	"encoding/json"
	"time"

	"github.com/gin-gonic/gin"
)

func windowTime(c *gin.Context, field string, defaultValue time.Time) (time.Time, error) {
	raw, exists := c.GetQuery(field)
	if !exists {
		return defaultValue, nil
	}
	if date, err := time.ParseInLocation(time.DateOnly, raw, beijing); err == nil {
		return date, nil
	}
	value, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return time.Time{}, invalid(field)
	}
	return value, nil
}

func (h handler) runsStats(c *gin.Context) {
	now := time.Now().In(beijing)
	to, err := windowTime(c, "to", now)
	if err != nil {
		apiFailure(c, err)
		return
	}
	from, err := windowTime(c, "from", to.AddDate(0, 0, -7))
	if err != nil {
		apiFailure(c, err)
		return
	}
	if !from.Before(to) {
		apiFailure(c, invalid("from"))
		return
	}
	h.read(c, "RunsStats", from, to)
}

func (h handler) knowledgeDelta(c *gin.Context) {
	now := time.Now().In(beijing)
	date := c.DefaultQuery("date", now.Format(time.DateOnly))
	start, err := time.ParseInLocation(time.DateOnly, date, beijing)
	if err != nil || start.After(now) {
		apiFailure(c, invalid("date"))
		return
	}
	h.read(c, "KnowledgeDelta", start, start.AddDate(0, 0, 1))
}

func (h handler) trends(c *gin.Context) {
	metric := c.Query("metric")
	var query string
	switch metric {
	case "knowledge_growth":
		query = "KnowledgeTrend"
	case "ai_cost":
		query = "CostTrend"
	default:
		apiFailure(c, invalid("metric"))
		return
	}
	days, err := pageNumber(c, "days", 7, 1, 90)
	if err != nil {
		apiFailure(c, err)
		return
	}
	start := time.Now().In(beijing).AddDate(0, 0, 1-days).Format(time.DateOnly)
	points, err := h.readJSON(c, query, start, days)
	if err != nil {
		apiFailure(c, err)
		return
	}
	c.JSON(200, gin.H{"points": json.RawMessage(points)})
}
