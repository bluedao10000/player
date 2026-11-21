package knowledge_test

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	routing "anime-agent/api/internal/router"
	"anime-agent/api/internal/testsupport"
	sqltest "anime-agent/database/queries/fixtures"
)

func TestLastVerifiedBackupSurvivesHistoryLimit(t *testing.T) {
	db := testsupport.Database(t)
	_, err := db.Exec(context.Background(), sqltest.SeedBackupHistory)
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	routing.Register(router, db)
	token := "12345678901234567890123456789012"
	t.Setenv("KNOWLEDGE_ADMIN_TOKEN", token)
	t.Setenv("KNOWLEDGE_ADMIN_OPERATOR", "fixture")
	req := httptest.NewRequest("GET", "/api/v1/admin/knowledge/backup-status", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Fatal(w.Code)
	}
	w = httptest.NewRecorder()
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)
	var result struct {
		Latest struct {
			Name string `json:"job_name"`
		} `json:"latest_verified"`
		Runs []struct {
			Status   string
			Attempts []struct{ Reason string }
		}
	}
	if err = json.Unmarshal(w.Body.Bytes(), &result); err != nil || w.Code != 200 || result.Latest.Name != "verified" || len(result.Runs) != 30 || result.Runs[0].Status != "failed" {
		t.Fatal(w.Code, result, err)
	}
	attempts := result.Runs[0].Attempts
	if len(attempts) != 1 || attempts[0].Reason != "快照导出失败" {
		t.Fatal(attempts)
	}
}
