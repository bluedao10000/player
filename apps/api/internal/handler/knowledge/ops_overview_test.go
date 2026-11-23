package knowledge_test

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	routing "anime-agent/api/internal/router"
	"anime-agent/api/internal/testsupport"
	sqltest "anime-agent/api/internal/testsupport/fixtures"
)

func TestOpsOverviewAggregatesSnapshotHeartbeatAndBeijingDay(t *testing.T) {
	db := testsupport.Database(t)
	ctx := context.Background()
	for _, statement := range []string{
		sqltest.SeedOpsOverviewSnapshot,
		sqltest.SeedOpsOverviewHeartbeats,
		sqltest.SeedOpsOverviewRuns,
		sqltest.SeedOpsEditors,
	} {
		if _, err := db.Exec(ctx, statement); err != nil {
			t.Fatalf("overview fixture failed: %v\n%s", err, statement)
		}
	}
	var expectedRaw []byte
	if err := db.QueryRow(ctx, sqltest.OpsOverviewExpectedLiveMetrics).Scan(&expectedRaw); err != nil {
		t.Fatal(err)
	}
	var expected struct {
		Metrics  map[string]int `json:"metrics"`
		Boundary map[string]int `json:"boundary"`
	}
	if err := json.Unmarshal(expectedRaw, &expected); err != nil {
		t.Fatal(err)
	}

	router := gin.New()
	routing.Register(router, db)
	token := "12345678901234567890123456789012"
	t.Setenv("KNOWLEDGE_ADMIN_TOKEN", token)
	t.Setenv("KNOWLEDGE_ADMIN_OPERATOR", "fixture")

	request := httptest.NewRequest("GET", "/api/v1/admin/knowledge/ops/overview", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != 401 {
		t.Fatalf("unauthenticated overview status=%d", recorder.Code)
	}
	request = httptest.NewRequest("GET", "/api/v1/admin/knowledge/ops/overview", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != 200 {
		t.Fatalf("overview status=%d body=%s", recorder.Code, recorder.Body.String())
	}

	var result struct {
		AsOf      string                    `json:"as_of"`
		Metrics   map[string]overviewMetric `json:"metrics"`
		Attention []overviewAttentionItem   `json:"attention"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode overview response: %v\n%s", err, recorder.Body.String())
	}
	if result.AsOf == "" {
		t.Fatal("overview omitted as_of")
	}
	if got, want := len(result.Metrics), 18; got != want {
		t.Fatalf("metric count=%d want=%d: %+v", got, want, result.Metrics)
	}
	for _, name := range []string{
		"active_works", "published_worlds", "work_types", "characters", "portraits", "chapters",
		"chapter_images", "storylines", "scenes", "locations", "online_workers", "running_executions",
		"running_tasks", "pending_work", "blocked_work", "waiting_human", "completed_today", "failed_today",
	} {
		metric, ok := result.Metrics[name]
		wantDefinition := 1
		if name == "running_executions" {
			wantDefinition = 2
		}
		if !ok || metric.Definition != wantDefinition || metric.UpdatedAt == "" || len(metric.Value) == 0 {
			t.Errorf("metric %s missing value/definition/update time: %+v", name, metric)
		}
	}
	for name, want := range map[string]int{
		"active_works": 23, "published_worlds": 18, "characters": 10, "portraits": 20,
		"chapters": 30, "chapter_images": 40, "storylines": 50, "scenes": 60, "locations": 70,
	} {
		var got int
		if err := json.Unmarshal(result.Metrics[name].Value, &got); err != nil || got != want {
			t.Errorf("metric %s=%d, want %d (decode err=%v)", name, got, want, err)
		}
	}
	for name, want := range expected.Metrics {
		var got int
		if err := json.Unmarshal(result.Metrics[name].Value, &got); err != nil || got != want {
			t.Errorf("metric %s=%d, source views report %d (decode err=%v)", name, got, want, err)
		}
	}
	for name, want := range map[string]int{"success_before": 1, "success_today": 1, "failed_before": 1, "failed_today": 1} {
		if expected.Boundary[name] != want {
			t.Errorf("Beijing day boundary fixture %s=%d want=%d", name, expected.Boundary[name], want)
		}
	}
	var types []struct {
		Type  *string `json:"type"`
		Count int     `json:"count"`
	}
	if err := json.Unmarshal(result.Metrics["work_types"].Value, &types); err != nil {
		t.Fatal(err)
	}
	if len(types) != 2 || types[0].Type == nil || *types[0].Type != "TV" || types[0].Count != 2 || types[1].Type != nil || types[1].Count != 1 {
		t.Fatalf("work type distribution=%+v", types)
	}

	if len(result.Attention) < 5 || len(result.Attention) > 20 {
		t.Fatalf("attention count=%d, want 5..20: %+v", len(result.Attention), result.Attention)
	}
	if result.Attention[0].Severity != "critical" || result.Attention[0].ReasonCode != "worker_offline" {
		t.Fatalf("highest severity item=%+v", result.Attention[0])
	}
	seen := map[string]bool{}
	for _, item := range result.Attention {
		seen[item.Kind+":"+item.ReasonCode] = true
		if item.Stage == "" || item.Since == "" || item.Link.Page == "" || item.Link.Params == nil {
			t.Errorf("attention item missing detail/navigation fields: %+v", item)
		}
	}
	for _, key := range []string{"worker:worker_offline", "worker:worker_consecutive_errors", "work_item:awaiting_review", "work_item:draft_conflict", "failed_run:execution_failed"} {
		if !seen[key] {
			t.Errorf("attention missing %s: %+v", key, result.Attention)
		}
	}
}

func TestOpsAttentionReturnsFullListAndOverviewCapsAtTwenty(t *testing.T) {
	db := testsupport.Database(t)
	if _, err := db.Exec(context.Background(), sqltest.SeedOpsOverviewManyOffline); err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	routing.Register(router, db)
	token := "12345678901234567890123456789012"
	t.Setenv("KNOWLEDGE_ADMIN_TOKEN", token)
	t.Setenv("KNOWLEDGE_ADMIN_OPERATOR", "fixture")
	for _, tc := range []struct {
		path string
		want int
	}{{"/api/v1/admin/knowledge/ops/attention", 25}, {"/api/v1/admin/knowledge/ops/overview", 20}} {
		request := httptest.NewRequest("GET", tc.path, nil)
		request.Header.Set("Authorization", "Bearer "+token)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		var result struct {
			Attention []overviewAttentionItem `json:"attention"`
		}
		if recorder.Code != 200 || json.Unmarshal(recorder.Body.Bytes(), &result) != nil {
			t.Fatalf("%s status=%d body=%s", tc.path, recorder.Code, recorder.Body.String())
		}
		if len(result.Attention) != tc.want {
			t.Errorf("%s attention count=%d want=%d", tc.path, len(result.Attention), tc.want)
		}
		for _, item := range result.Attention {
			if item.Severity != "critical" {
				t.Errorf("attention is not ordered by severity: %+v", item)
				break
			}
		}
	}
}

type overviewMetric struct {
	Value      json.RawMessage `json:"value"`
	Definition int             `json:"definition"`
	UpdatedAt  string          `json:"updated_at"`
}

type overviewAttentionItem struct {
	Kind       string `json:"kind"`
	Severity   string `json:"severity"`
	Stage      string `json:"stage"`
	ReasonCode string `json:"reason_code"`
	Since      string `json:"since"`
	Link       struct {
		Page   string         `json:"page"`
		Params map[string]any `json:"params"`
	} `json:"link"`
}
