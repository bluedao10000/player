package knowledge

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestLastVerifiedBackupSurvivesHistoryLimit(t *testing.T) {
	db := testStore(t)
	_, err := db.Exec(context.Background(), `INSERT INTO backup_runs VALUES(gen_random_uuid(),'verified',now()-interval '32 days',now()-interval '31 days','success',123,92,NULL);
 INSERT INTO backup_runs SELECT gen_random_uuid(),'failed-'||i,now()-make_interval(days=>i),now()-make_interval(days=>i),'failed',NULL,NULL,'连接中断' FROM generate_series(1,31) i;
 INSERT INTO backup_failed_attempts SELECT gen_random_uuid(),job_uid,started_at,'快照导出失败' FROM backup_runs WHERE job_name='failed-1';`)
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	Routes(router, db)
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
