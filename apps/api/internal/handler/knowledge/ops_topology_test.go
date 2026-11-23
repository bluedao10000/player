package knowledge_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"

	opsread "anime-agent/api/internal/repository/queries/ops"
	routing "anime-agent/api/internal/router"
	"anime-agent/api/internal/testsupport"
	sqltest "anime-agent/api/internal/testsupport/fixtures"
)

func TestOpsTopologyGroupsStageMetricsWorldFilterAndSixStateProgress(t *testing.T) {
	db := testsupport.Database(t)
	ctx := context.Background()
	for _, statement := range []string{
		sqltest.SeedOpsSyncRuns, sqltest.SeedOpsWorlds, sqltest.SeedOpsQualityRuns,
		sqltest.SeedOpsProductionRequests, sqltest.SeedOpsConfigurationRuns,
		sqltest.SeedOpsEpisodeRuns, sqltest.SeedOpsTopologyAcceptanceHeads, sqltest.SeedOpsPolicyItems,
		sqltest.SeedOpsPolicyAdmissions, sqltest.SeedOpsSourceCapture,
		sqltest.SeedOpsEditors, sqltest.SeedOpsMediaSteps,
		sqltest.SeedOpsMediaCandidates, sqltest.SeedOpsBackups,
		sqltest.SeedOpsTopologyProgress, sqltest.SeedOpsTopologyOverlappingWorld, sqltest.SeedOpsTopologyHeartbeats,
	} {
		if _, err := db.Exec(ctx, statement); err != nil {
			t.Fatalf("topology fixture failed: %v\n%s", err, statement)
		}
	}

	body, err := opsread.Topology(ctx, db, "24h", 24, "tidepost")
	if err != nil {
		t.Fatalf("load topology: %s", topologyQueryError(err))
	}
	var result topologyResponse
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("decode topology: %v\n%s", err, body)
	}
	if result.Timezone != "Asia/Shanghai" || result.Window != "24h" || result.AsOf == "" || len(result.Groups) != 5 {
		t.Fatalf("topology metadata/groups = %+v", result)
	}
	if result.SelectedWorld == nil || result.SelectedWorld.Slug != "tidepost" || !result.SelectedWorld.HasRecords {
		t.Fatalf("selected world = %+v", result.SelectedWorld)
	}
	if len(result.Infrastructure.Stages) != 1 || result.Infrastructure.Stages[0].ID != "backup" || result.Infrastructure.Stages[0].TopologyGroup != nil || !result.Infrastructure.Stages[0].GlobalScope {
		t.Fatalf("infrastructure stage = %+v", result.Infrastructure.Stages)
	}

	stages := map[string]topologyStage{}
	for _, group := range result.Groups {
		if len(group.Stages) == 0 {
			t.Errorf("topology group %s has no stages", group.ID)
		}
		for _, stage := range group.Stages {
			if stage.TopologyGroup == nil || *stage.TopologyGroup != group.ID {
				t.Errorf("stage %s group=%v, parent=%s", stage.ID, stage.TopologyGroup, group.ID)
			}
			stages[stage.ID] = stage
		}
	}
	stages["backup"] = result.Infrastructure.Stages[0]
	if len(stages) != 17 {
		t.Fatalf("stage count=%d want=17", len(stages))
	}
	for _, id := range []string{"source_sync", "normalize", "quality_scan", "inventory", "plan", "configuration", "admission", "capture", "research", "editorial", "media_reference", "media_generate", "media_review", "media_rework", "acceptance", "release", "backup"} {
		if _, ok := stages[id]; !ok {
			t.Errorf("missing stage definition %s", id)
		}
	}
	for _, id := range []string{"normalize", "inventory"} {
		if stages[id].Tracked || stages[id].Current != nil || stages[id].WindowStats != nil || stages[id].Agents != nil {
			t.Errorf("untracked stage %s has metrics: %+v", id, stages[id])
		}
	}

	source := stages["source_sync"]
	if source.Current == nil || source.Current.Total != 1 || source.Current.Running != 1 || source.Current.Pending != 0 {
		t.Errorf("source current state=%+v", source.Current)
	}
	if source.WindowStats == nil || source.WindowStats.Success != 1 || source.WindowStats.Failed != 1 || source.WindowStats.Skipped != 1 || source.WindowStats.Completed != 3 || source.WindowStats.SuccessRate == nil || math.Abs(*source.WindowStats.SuccessRate-.5) > 0.00001 {
		t.Errorf("source window metrics=%+v", source.WindowStats)
	}
	if source.Agents == nil || source.Agents.OnlineWorkers == nil || *source.Agents.OnlineWorkers != 1 || source.Agents.RunningExecutions != nil {
		t.Errorf("source worker metrics=%+v", source.Agents)
	}
	if stages["capture"].Current == nil || stages["capture"].Current.Running != 1 || !stages["capture"].HasRecords {
		t.Errorf("world-scoped capture state=%+v", stages["capture"])
	}
	if stages["backup"].WindowStats == nil || stages["backup"].WindowStats.Completed != 2 || !stages["backup"].GlobalScope {
		t.Errorf("global backup metrics=%+v", stages["backup"])
	}
	if stages["media_reference"].Agents == nil || stages["media_reference"].Agents.RunningExecutions == nil || *stages["media_reference"].Agents.RunningExecutions != 2 {
		t.Errorf("media reference Agent count included legacy/offline counters: %+v", stages["media_reference"].Agents)
	}
	if stages["media_generate"].Agents == nil || stages["media_generate"].Agents.RunningExecutions == nil || *stages["media_generate"].Agents.RunningExecutions != 3 {
		t.Errorf("media generate Agent count=%+v", stages["media_generate"].Agents)
	}
	if stages["media_review"].Agents == nil || stages["media_review"].Agents.RunningExecutions == nil || *stages["media_review"].Agents.RunningExecutions != 4 {
		t.Errorf("media review Agent count=%+v", stages["media_review"].Agents)
	}
	if stages["media_rework"].Agents == nil || stages["media_rework"].Agents.RunningExecutions == nil || *stages["media_rework"].Agents.RunningExecutions != 5 {
		t.Errorf("media rework Agent count=%+v", stages["media_rework"].Agents)
	}
	if stages["plan"].Agents == nil || stages["plan"].Agents.RunningExecutions == nil || *stages["plan"].Agents.RunningExecutions != 6 {
		t.Errorf("planning Agent count=%+v", stages["plan"].Agents)
	}
	if stages["configuration"].Agents == nil || stages["configuration"].Agents.RunningExecutions == nil || *stages["configuration"].Agents.RunningExecutions != 7 {
		t.Errorf("configuration Agent count=%+v", stages["configuration"].Agents)
	}

	progress := result.SelectedWorld.Progress
	if progress.ScopeTotal != 15 || progress.Unregistered != 3 || progress.Registered != 4 || progress.Planned != 1 || progress.InProduction != 3 || progress.Accepted != 3 || progress.Published != 1 {
		t.Fatalf("six-state progress=%+v", progress)
	}
	if sum := progress.Unregistered + progress.Registered + progress.Planned + progress.InProduction + progress.Accepted + progress.Published; sum != progress.ScopeTotal {
		t.Fatalf("six states sum=%d scope=%d", sum, progress.ScopeTotal)
	}
	var overlapCount int
	if err := db.QueryRow(ctx, sqltest.OpsOverlappingPolicyRangeDedup).Scan(&overlapCount); err != nil || overlapCount != 6 {
		t.Fatalf("overlapping strategy range distinct count=%d err=%v", overlapCount, err)
	}
}

func TestOpsTopologyUsesBeijingWindowBoundsAndDurationPercentiles(t *testing.T) {
	db := testsupport.Database(t)
	ctx := context.Background()
	asOf := time.Date(2026, 11, 23, 0, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	if _, err := db.Exec(ctx, sqltest.SeedOpsTopologyBoundaryRuns, asOf); err != nil {
		t.Fatalf("boundary fixture: %v", err)
	}
	rows, err := db.Query(ctx, sqltest.OpsTopologyBoundaryEvents, asOf)
	if err != nil {
		t.Fatal(err)
	}
	inWindow := 0
	for rows.Next() {
		var finishedAt time.Time
		var afterStart, beforeEnd bool
		if err := rows.Scan(&finishedAt, &afterStart, &beforeEnd); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		if afterStart && beforeEnd {
			inWindow++
		}
	}
	rows.Close()
	if inWindow != 4 {
		t.Fatalf("fixture includes %d runs in [%s-24h,%s), want 4", inWindow, asOf, asOf)
	}
	body, err := opsread.TopologyAt(ctx, db, "24h", 24, "", &asOf)
	if err != nil {
		t.Fatalf("query fixed topology window: %s", topologyQueryError(err))
	}
	var result topologyResponse
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("decode topology: %v\n%s", err, body)
	}
	var source topologyStage
	for _, group := range result.Groups {
		for _, stage := range group.Stages {
			if stage.ID == "source_sync" {
				source = stage
			}
		}
	}
	if source.WindowStats == nil || source.WindowStats.Success != 4 || source.WindowStats.Duration.SampleCount != 4 {
		t.Fatalf("left-closed/right-open window count=%+v", source.WindowStats)
	}
	if source.WindowStats.Duration.AverageSeconds == nil || math.Abs(*source.WindowStats.Duration.AverageSeconds-32.75) > .001 ||
		source.WindowStats.Duration.MedianSeconds == nil || math.Abs(*source.WindowStats.Duration.MedianSeconds-15) > .001 ||
		source.WindowStats.Duration.P90Seconds == nil || math.Abs(*source.WindowStats.Duration.P90Seconds-76) > .001 {
		t.Fatalf("duration percentiles=%+v", source.WindowStats.Duration)
	}
	if source.WindowStats.DurationPrimary != "median" {
		t.Fatalf("duration primary=%q, want median", source.WindowStats.DurationPrimary)
	}
	start := asOf.Add(-24 * time.Hour)
	if !result.WindowStart.Equal(start) || !result.WindowEnd.Equal(asOf) {
		t.Fatalf("Beijing midnight window=[%s,%s), want [%s,%s)", result.WindowStart, result.WindowEnd, start, asOf)
	}
}

func topologyQueryError(err error) string {
	var postgres *pgconn.PgError
	if errors.As(err, &postgres) {
		return fmt.Sprintf("%v (position %d, detail %s)", err, postgres.Position, postgres.Detail)
	}
	return err.Error()
}

func TestOpsTopologyRoutesAndStageDetailPagination(t *testing.T) {
	db := testsupport.Database(t)
	ctx := context.Background()
	for _, statement := range []string{sqltest.SeedOpsSyncRuns, sqltest.SeedOpsWorlds} {
		if _, err := db.Exec(ctx, statement); err != nil {
			t.Fatalf("topology route fixture: %v", err)
		}
	}
	router := gin.New()
	routing.Register(router, db)
	t.Setenv("KNOWLEDGE_ADMIN_TOKEN", "12345678901234567890123456789012")
	t.Setenv("KNOWLEDGE_ADMIN_OPERATOR", "fixture")
	request := func(path string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest("GET", path, nil)
		req.Header.Set("Authorization", "Bearer 12345678901234567890123456789012")
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)
		return recorder
	}

	response := request("/api/v1/admin/knowledge/ops/topology?window=7d&world=tidepost")
	if response.Code != 200 {
		t.Fatalf("topology status=%d body=%s", response.Code, response.Body.String())
	}
	var topology topologyResponse
	if err := json.Unmarshal(response.Body.Bytes(), &topology); err != nil || topology.Window != "7d" || topology.SelectedWorld == nil || topology.SelectedWorld.Slug != "tidepost" {
		t.Fatalf("topology response=%+v err=%v", topology, err)
	}
	if got := request("/api/v1/admin/knowledge/ops/topology?window=3d"); got.Code != 400 {
		t.Errorf("invalid window status=%d body=%s", got.Code, got.Body.String())
	}
	if got := request("/api/v1/admin/knowledge/ops/topology?world=missing-world"); got.Code != 404 {
		t.Errorf("unknown world status=%d body=%s", got.Code, got.Body.String())
	}

	response = request("/api/v1/admin/knowledge/ops/topology/stages/source_sync?limit=1")
	if response.Code != 200 {
		t.Fatalf("stage detail status=%d body=%s", response.Code, response.Body.String())
	}
	var first struct {
		RecentRuns []json.RawMessage `json:"recent_runs"`
		Pagination struct {
			RunsAfter string `json:"runs_after"`
		} `json:"pagination"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &first); err != nil || len(first.RecentRuns) != 1 || first.Pagination.RunsAfter == "" {
		t.Fatalf("first detail page=%+v err=%v body=%s", first, err, response.Body.String())
	}
	response = request("/api/v1/admin/knowledge/ops/topology/stages/source_sync?limit=1&runs_after=" + first.Pagination.RunsAfter)
	var second struct {
		RecentRuns []json.RawMessage `json:"recent_runs"`
	}
	if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &second) != nil || len(second.RecentRuns) != 1 {
		t.Fatalf("second detail page status=%d body=%s", response.Code, response.Body.String())
	}
	if got := request("/api/v1/admin/knowledge/ops/topology/stages/unknown"); got.Code != 404 {
		t.Errorf("unknown stage status=%d body=%s", got.Code, got.Body.String())
	}
	if got := request("/api/v1/admin/knowledge/ops/topology/stages/source_sync?queue_after=invalid"); got.Code != 400 {
		t.Errorf("invalid cursor status=%d body=%s", got.Code, got.Body.String())
	}
}

type topologyResponse struct {
	AsOf        string    `json:"as_of"`
	Timezone    string    `json:"timezone"`
	Window      string    `json:"window"`
	WindowStart time.Time `json:"window_start"`
	WindowEnd   time.Time `json:"window_end"`
	Groups      []struct {
		ID     string          `json:"id"`
		Stages []topologyStage `json:"stages"`
	} `json:"groups"`
	SelectedWorld *struct {
		Slug       string `json:"slug"`
		HasRecords bool   `json:"has_records"`
		Progress   struct {
			ScopeTotal   int `json:"scope_total"`
			Unregistered int `json:"unregistered"`
			Registered   int `json:"registered"`
			Planned      int `json:"planned"`
			InProduction int `json:"in_production"`
			Accepted     int `json:"accepted"`
			Published    int `json:"published"`
		} `json:"progress"`
	} `json:"selected_world"`
	Infrastructure struct {
		Stages []topologyStage `json:"stages"`
	} `json:"infrastructure"`
}

type topologyStage struct {
	ID            string  `json:"id"`
	Tracked       bool    `json:"tracked"`
	TopologyGroup *string `json:"topology_group"`
	HasRecords    bool    `json:"has_records"`
	GlobalScope   bool    `json:"global_scope"`
	Current       *struct {
		Total        int `json:"total"`
		Running      int `json:"running"`
		Pending      int `json:"pending"`
		Blocked      int `json:"blocked"`
		WaitingHuman int `json:"waiting_human"`
	} `json:"current"`
	WindowStats *struct {
		Success     int      `json:"success"`
		Failed      int      `json:"failed"`
		Skipped     int      `json:"skipped"`
		Completed   int      `json:"completed"`
		SuccessRate *float64 `json:"success_rate"`
		Duration    struct {
			SampleCount    int      `json:"sample_count"`
			AverageSeconds *float64 `json:"average_seconds"`
			MedianSeconds  *float64 `json:"median_seconds"`
			P90Seconds     *float64 `json:"p90_seconds"`
		} `json:"duration"`
		DurationPrimary string `json:"duration_primary"`
	} `json:"window_stats"`
	Agents *struct {
		Mapped            bool   `json:"mapped"`
		OnlineWorkers     *int   `json:"online_workers"`
		RunningExecutions *int   `json:"running_executions"`
		ExecutionCoverage string `json:"execution_coverage"`
	} `json:"agents"`
}
