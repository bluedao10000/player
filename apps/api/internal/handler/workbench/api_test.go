package workbench_test

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"anime-agent/api/internal/router"
	"anime-agent/api/internal/service/knowledge"
	"anime-agent/api/internal/testsupport"
	"anime-agent/api/internal/testsupport/fixtures"
)

const operatorToken = "0123456789abcdef0123456789abcdef"
const apiPrefix = "/api/v1"

type object = map[string]any

func setOperator(t *testing.T) {
	t.Helper()
	t.Setenv("KNOWLEDGE_ADMIN_TOKEN", operatorToken)
	t.Setenv("KNOWLEDGE_ADMIN_OPERATOR", "fixture")
}

func request(method, path, body string) *http.Request {
	req := httptest.NewRequest(method, apiPrefix+path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+operatorToken)
	req.Header.Set("Content-Type", "application/json")
	return req
}

func jsonResponse[T any](t *testing.T, r *gin.Engine, method, path, body string, want int) T {
	t.Helper()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, request(method, path, body))
	if w.Code != want {
		t.Fatalf("%s %s = %d, want %d: %s", method, path, w.Code, want, w.Body)
	}
	var out T
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("%s: %v: %s", path, err, w.Body)
	}
	return out
}

func TestHTTPContractValidationAndAuthorization(t *testing.T) {
	setOperator(t)
	r := gin.New()
	router.Register(r, nil)
	response := httptest.NewRecorder()
	r.ServeHTTP(response, httptest.NewRequest("GET", apiPrefix+"/briefing", nil))
	if response.Code != 401 || !strings.Contains(response.Body.String(), `"code":"unauthenticated"`) {
		t.Fatalf("unauthenticated: %d %s", response.Code, response.Body)
	}
	for _, path := range []string{"/briefing?date=invalid", "/stats/knowledge-delta?date=invalid", "/works?page=-1", "/works?page_size=101", "/decisions?risk=L4", "/runs?status=posted"} {
		err := jsonResponse[object](t, r, "GET", path, "", 400)["error"].(object)
		if err["code"] != "invalid_argument" || err["details"].(object)["field"] == nil {
			t.Fatalf("%s: %v", path, err)
		}
	}
}

func TestWorkbenchReadsAndSourceApproval(t *testing.T) {
	db := testsupport.Database(t)
	setOperator(t)
	ctx := context.Background()
	if err := knowledge.Sweep(ctx, db, "tidepost-original", true, false); err != nil {
		t.Fatal(err)
	}
	if err := knowledge.RecalculateCompleteness(ctx, db); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{fixtures.SeedOpsSyncRuns, fixtures.SeedOpsSyncRuns, fixtures.Workbench["SourceMark"]} {
		if _, err := db.Exec(ctx, name); err != nil {
			t.Fatal(err)
		}
	}
	r := gin.New()
	router.Register(r, db)
	now := time.Now().In(time.FixedZone("Beijing", 8*3600))
	if _, err := db.Exec(ctx, fixtures.Workbench["SeedKnowledgeTrend"], now.Format(time.DateOnly)); err != nil {
		t.Fatal(err)
	}
	brief := jsonResponse[object](t, r, "GET", "/briefing", "", 200)
	if !strings.HasSuffix(brief["generated_at"].(string), "+08:00") ||
		!strings.Contains(strings.Join(strings.Fields(fmt.Sprint(brief["summary_tokens"])), ""), "手动停止") {
		t.Fatal(brief)
	}
	for _, item := range brief["metrics"].([]any) {
		metric := item.(object)
		if metric["note"] == "" {
			t.Fatal(metric)
		}
		if metric["key"] == "verified_items" &&
			(metric["value"].(float64) <= 0 || metric["delta"] != "+3" || metric["deltaGood"] != true) {
			t.Fatal(metric)
		}
	}
	execution := brief["execution"].(object)
	executionTotal := 0.0
	for _, raw := range execution["by_type"].([]any) {
		group := raw.(object)
		if group["task_type"] == "" || len(group["by_status"].(object)) == 0 {
			t.Fatal(group)
		}
		executionTotal += group["total"].(float64)
	}
	if execution["total"] == float64(0) || execution["total"] != executionTotal {
		t.Fatal(execution)
	}
	failureRisks := 0
	for _, raw := range brief["risks"].([]any) {
		risk := raw.(object)
		if risk["error_summary"] == "fixture source error" {
			failureRisks++
			if risk["occurrences"] != float64(2) || risk["detail"] != "fixture source error" {
				t.Fatal(risk)
			}
		}
	}
	if failureRisks != 1 {
		t.Fatalf("risks=%v", brief["risks"])
	}
	page := jsonResponse[object](t, r, "GET", "/works?search=潮汐邮局&status=published&sort=coverage", "", 200)
	if page["total"] != float64(1) {
		t.Fatal(page)
	}
	work := page["rows"].([]any)[0].(object)
	workPath := "/works/" + work["work_id"].(string)
	if work["coverage"] != nil || work["verified_entity_count"].(float64) <= 0 || work["chapter_count"] != float64(4) {
		t.Fatal(work)
	}
	jsonResponse[object](t, r, "GET", workPath, "", 200)
	profile := jsonResponse[object](t, r, "GET", workPath+"/profile", "", 200)
	inline := work["workbench_profile"].(object)
	if profile["completeness"] == nil || profile["completeness"] != inline["completeness"] ||
		profile["verified_entity_count"] != inline["verified_entity_count"] || profile["gap_count"] != inline["gap_count"] {
		t.Fatalf("profile=%v row=%v", profile, inline)
	}
	overview := jsonResponse[object](t, r, "GET", "/stats/overview", "", 200)
	if overview["published_versions"] != float64(1) || overview["verified_items"].(float64) < profile["verified_entity_count"].(float64) ||
		overview["open_gaps"].(float64) <= 0 || overview["avg_completeness"] != profile["completeness"] {
		t.Fatal(overview)
	}
	allRuns := jsonResponse[[]object](t, r, "GET", "/runs", "", 200)
	runIDs := make(map[any]bool)
	for _, run := range allRuns {
		runIDs[run["run_id"]] = true
	}
	delta := jsonResponse[object](t, r, "GET", "/stats/knowledge-delta?date="+now.Format(time.DateOnly), "", 200)
	rows := delta["rows"].([]any)
	if len(rows) != 1 {
		t.Fatal(delta)
	}
	deltaRow := rows[0].(object)
	byKind := deltaRow["by_kind"].(object)
	entityCount := 0.0
	for _, kind := range []string{"character", "chapter", "scene"} {
		if value, ok := byKind[kind].(float64); ok {
			entityCount += value
		}
	}
	if deltaRow["work_id"] != work["work_id"] || deltaRow["new_verified_items"] != overview["verified_items"] ||
		entityCount != profile["verified_entity_count"] || len(deltaRow["source_run_ids"].([]any)) == 0 {
		t.Fatalf("delta=%v overview=%v profile=%v", delta, overview, profile)
	}
	for _, id := range deltaRow["source_run_ids"].([]any) {
		if !runIDs[id] {
			t.Fatalf("unknown source run %v", id)
		}
	}
	pastDelta := jsonResponse[object](t, r, "GET", "/stats/knowledge-delta?date="+now.AddDate(0, 0, -1).Format(time.DateOnly), "", 200)
	if len(pastDelta["rows"].([]any)) != 0 {
		t.Fatal(pastDelta)
	}
	historical := jsonResponse[object](t, r, "GET", "/briefing?date="+now.AddDate(0, 0, -1).Format(time.DateOnly), "", 200)
	for _, item := range historical["metrics"].([]any) {
		metric := item.(object)
		if metric["key"] == "verified_items" && (metric["delta"] != "-3" || metric["deltaGood"] != false) {
			t.Fatal(metric)
		}
	}
	trend := jsonResponse[object](t, r, "GET", "/trends?metric=knowledge_growth&days=2", "", 200)["points"].([]any)
	if trend[0].(object)["value"] != nil || trend[1].(object)["value"] != float64(3) {
		t.Fatal(trend)
	}
	gaps := jsonResponse[[]object](t, r, "GET", workPath+"/gaps", "", 200)
	recs := jsonResponse[[]object](t, r, "GET", workPath+"/recommendations", "", 200)
	if len(gaps) == 0 || len(recs) != len(gaps) || recs[0]["estimated_cost"] != nil {
		t.Fatalf("gaps=%v recommendations=%v", gaps, recs)
	}
	missing := jsonResponse[object](t, r, "GET", "/works?search=not-found-fixture", "", 200)
	if missing["total"] != float64(0) || len(missing["rows"].([]any)) != 0 {
		t.Fatal(missing)
	}
	items := jsonResponse[[]object](t, r, "GET", "/decisions?status=pending&risk=L2", "", 200)
	if len(items) != 1 || items[0]["diff"] != nil {
		t.Fatal(items)
	}
	id := items[0]["id"].(string)
	path := "/decisions/" + id
	decision := jsonResponse[object](t, r, "GET", path, "", 200)
	if diff := decision["diff"].([]any); len(diff) != 1 || diff[0].(object)["before"] != "旧标题" {
		t.Fatal(decision)
	}
	evidence := jsonResponse[[]object](t, r, "GET", "/evidence", "", 200)
	evidenceIDs := make(map[any]bool)
	for _, item := range evidence {
		evidenceIDs[item["id"]] = true
	}
	recommendations := 0
	for _, raw := range brief["actions"].([]any) {
		action := raw.(object)
		if action["intent"] == "" {
			t.Fatal(action)
		}
		for _, ref := range action["evidence_refs"].([]any) {
			if !evidenceIDs[ref] {
				t.Fatalf("missing evidence %v for %v", ref, action)
			}
		}
		if action["decision_id"] == nil {
			recommendations++
		}
	}
	if len(evidenceIDs) < 2 || recommendations == 0 {
		t.Fatalf("evidence=%v actions=%v", evidence, brief["actions"])
	}
	if _, err := db.Exec(ctx, fixtures.Workbench["RefreshSourceMark"]); err != nil {
		t.Fatal(err)
	}
	resolved := path + "/resolve"
	stale := jsonResponse[object](t, r, "POST", resolved, `{"action":"approve","expected_revision":1}`, 409)["error"].(object)
	if stale["details"].(object)["current_revision"] != float64(2) {
		t.Fatal(stale)
	}
	jsonResponse[object](t, r, "POST", resolved, `{"action":"reject","expected_revision":2}`, 400)
	snoozed := jsonResponse[object](t, r, "POST", resolved, `{"action":"snooze","expected_revision":2}`, 200)
	if snoozed["status"] != "snoozed" || snoozed["expected_revision"] != float64(3) {
		t.Fatal(snoozed)
	}
	approved := jsonResponse[object](t, r, "POST", resolved, `{"action":"approve","expected_revision":3}`, 200)
	if approved["status"] != "approved" || approved["run_id"] == nil {
		t.Fatal(approved)
	}
	var closed bool
	if err := db.QueryRow(ctx, fixtures.Workbench["SourceResolved"]).Scan(&closed); err != nil || !closed {
		t.Fatalf("source closed=%v err=%v", closed, err)
	}
	var audited int
	if err := db.QueryRow(ctx, fixtures.Workbench["AuditCount"], id).Scan(&audited); err != nil || audited != 2 {
		t.Fatalf("audited %d: %v", audited, err)
	}
	runs := jsonResponse[[]object](t, r, "GET", "/runs?status=completed", "", 200)
	if len(runs) < 2 {
		t.Fatal(runs)
	}
	runPath := "/runs/" + approved["run_id"].(string)
	run := jsonResponse[object](t, r, "GET", runPath, "", 200)
	if run["related_decision_id"] != id || run["cost"] != nil {
		t.Fatal(run)
	}
	if events := jsonResponse[[]object](t, r, "GET", runPath+"/events", "", 200); len(events) != 0 {
		t.Fatal(events)
	}
	jsonResponse[object](t, r, "GET", "/runs/missing/events", "", 404)
}

func frames(t *testing.T, reader io.Reader) []object {
	t.Helper()
	events := []object{}
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line, ok := strings.CutPrefix(scanner.Text(), "data: ")
		if !ok {
			continue
		}
		var event object
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatal(err)
		}
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return events
}

func TestAgentPlanApprovalAndCancellation(t *testing.T) {
	db := testsupport.Database(t)
	setOperator(t)
	ctx := context.Background()
	if err := knowledge.Sweep(ctx, db, "tidepost-original", true, false); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, fixtures.Workbench["PriceFixture"]); err != nil {
		t.Fatal(err)
	}
	upstreamStopped := make(chan struct{}, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			Intent string
			Works  []struct {
				Work struct {
					ID string `json:"work_id"`
				}
			}
		}
		if json.NewDecoder(r.Body).Decode(&input) != nil {
			w.WriteHeader(400)
			return
		}
		io.WriteString(w, "{\"type\":\"token\",\"t\":\"已核对资料。\"}\n")
		w.(http.Flusher).Flush()
		if input.Intent == "停止测试" {
			<-r.Context().Done()
			upstreamStopped <- struct{}{}
			return
		}
		if len(input.Works) != 1 {
			io.WriteString(w, "{\"type\":\"error\"}\n")
			return
		}
		io.WriteString(w, "{\"type\":\"artifact\",\"artifact\":{\"type\":\"coverage-matrix\",\"ref\":\"discard-unverified-reference\"}}\n")
		fmt.Fprintf(w, `{"type":"plan","plan":{"work_id":%q,"title":"补全计划","objective":"核对角色出处","tasks":[{"title":"检查已有引用","priority":"high"}]}}`+"\n", input.Works[0].Work.ID)
		runID := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
		clock := time.Now().UTC().Format(time.RFC3339Nano)
		json.NewEncoder(w).Encode(object{
			"type": "done", "input_tokens": 20, "output_tokens": 9,
			"receipt": object{"id": runID, "subject": "workbench:" + runID, "account": "fixture",
				"endpoint": "http://workbench-model.test", "model": "folio-test", "purpose": "content",
				"operation": "conversation", "started": clock, "finished": clock, "outcome": "ok",
				"usage": object{"input_tokens": 20, "output_tokens": 9, "total_tokens": 29}},
		})
	}))
	defer upstream.Close()
	t.Setenv("KNOWLEDGE_RESEARCH_URL", upstream.URL)
	r := gin.New()
	router.Register(r, db)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, request("POST", "/agent/runs", `{"intent":"为潮汐邮局制定计划","model":"folio-agent","context":{"page":"knowledge"}}`))
	if w.Code != 200 || w.Header().Get("Content-Type") != "text/event-stream" {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	events := frames(t, w.Body)
	if len(events) < 7 || events[0]["type"] != "run_started" || events[len(events)-1]["type"] != "done" {
		t.Fatal(events)
	}
	id := events[0]["run_id"].(string)
	conversation := events[0]["conversation_id"].(string)
	artifacts := map[string]string{}
	for _, event := range events {
		if artifact, ok := event["artifact"].(object); ok {
			artifacts[artifact["type"].(string)] = artifact["ref"].(string)
		}
	}
	if artifacts["coverage-matrix"] != id {
		t.Fatal(artifacts)
	}
	workSet := jsonResponse[object](t, r, "GET", "/works?ref="+id, "", 200)
	if workSet["total"] != float64(1) {
		t.Fatal(workSet)
	}
	matrix := jsonResponse[object](t, r, "GET", "/artifacts/coverage-matrix/"+id, "", 200)
	if matrix["total"] != float64(1) || matrix["rows"].([]any)[0].(object)["dimensions"] == nil {
		t.Fatal(matrix)
	}
	runSet := jsonResponse[[]object](t, r, "GET", "/runs?ref="+id, "", 200)
	if len(runSet) != 1 || runSet[0]["run_id"] != id {
		t.Fatal(runSet)
	}
	planPath := "/plans/" + artifacts["plan"]
	decisionPath := "/decisions/" + artifacts["approval"]
	plan := jsonResponse[object](t, r, "GET", planPath, "", 200)
	if plan["status"] != "pending_approval" || plan["estimated_cost"] != nil || len(plan["tasks"].([]any)) != 1 {
		t.Fatal(plan)
	}
	runPath := "/agent/runs/" + id
	run := jsonResponse[object](t, r, "GET", runPath, "", 200)
	if run["status"] != "waiting_approval" || run["input_tokens"] != float64(20) || run["related_decision_id"] != artifacts["approval"] {
		t.Fatal(run)
	}
	if events[len(events)-1]["usage"].(object)["cost_usd"] != 0.000029 ||
		run["usage"].(object)["cost_usd"] != 0.000029 || run["currency"] != "USD" {
		t.Fatalf("costs: %v %v", events[len(events)-1], run)
	}
	var stored *string
	if err := db.QueryRow(ctx, fixtures.Workbench["MeasuredRunCost"], id).Scan(&stored); err != nil || stored == nil {
		t.Fatalf("saved cost=%v err=%v", stored, err)
	}
	stats := jsonResponse[object](t, r, "GET", "/runs/stats", "", 200)
	if stats["total_cost"] != 0.000029 || stats["currency"] != "USD" {
		t.Fatal(stats)
	}
	trend := jsonResponse[object](t, r, "GET", "/trends?metric=ai_cost&days=1", "", 200)
	if trend["points"].([]any)[0].(object)["value"] != 0.000029 {
		t.Fatal(trend)
	}
	jsonResponse[object](t, r, "POST", "/agent/runs/missing/cancel", "", 404)
	if replay := jsonResponse[[]object](t, r, "GET", "/runs/"+id+"/events", "", 200); len(replay) != len(events) {
		t.Fatalf("replay=%v events=%v", replay, events)
	}

	results := make(chan int, 2)
	for range 2 {
		go func() {
			response := httptest.NewRecorder()
			r.ServeHTTP(response, request("POST", decisionPath+"/resolve", `{"action":"approve","expected_revision":1}`))
			results <- response.Code
		}()
	}
	a, b := <-results, <-results
	if a+b != 200+409 || a == b {
		t.Fatalf("concurrent resolutions: %d, %d", a, b)
	}
	if plan = jsonResponse[object](t, r, "GET", planPath, "", 200); plan["status"] != "approved" || plan["expected_revision"] != float64(2) {
		t.Fatal(plan)
	}
	if run = jsonResponse[object](t, r, "GET", runPath, "", 200); run["status"] != "completed" {
		t.Fatal(run)
	}
	expireSetup := httptest.NewRecorder()
	r.ServeHTTP(expireSetup, request("POST", "/agent/runs", `{"intent":"为潮汐邮局制定另一个计划","model":"folio-agent"}`))
	for _, event := range frames(t, expireSetup.Body) {
		if artifact, ok := event["artifact"].(object); ok && artifact["type"] == "approval" {
			expiredID := artifact["ref"].(string)
			if _, err := db.Exec(ctx, fixtures.Workbench["ExpireDecision"], expiredID); err != nil {
				t.Fatal(err)
			}
			expired := jsonResponse[object](t, r, "POST", "/decisions/"+expiredID+"/resolve", `{"action":"approve","expected_revision":1}`, 410)
			if expired["error"].(object)["code"] != "decision_expired" {
				t.Fatal(expired)
			}
		}
	}

	api := httptest.NewServer(r)
	defer api.Close()
	reqCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, "POST", api.URL+apiPrefix+"/agent/runs",
		strings.NewReader(fmt.Sprintf(`{"intent":"停止测试","model":"folio-agent","conversation_id":%q}`, conversation)))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+operatorToken)
	req.Header.Set("Content-Type", "application/json")
	response, err := api.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var cancelledID string
	reader := bufio.NewScanner(response.Body)
	for reader.Scan() {
		var event object
		if data, ok := strings.CutPrefix(reader.Text(), "data: "); ok && json.Unmarshal([]byte(data), &event) == nil {
			if event["type"] == "run_started" {
				cancelledID = event["run_id"].(string)
			}
			if event["type"] == "token" {
				break
			}
		}
	}
	cancel()
	response.Body.Close()
	select {
	case <-upstreamStopped:
	case <-time.After(5 * time.Second):
		t.Fatal("upstream did not stop after SSE disconnect")
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		run = jsonResponse[object](t, r, "GET", "/agent/runs/"+cancelledID, "", 200)
		if run["status"] == "cancelled" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("cancelled run: %v", run)
		}
		time.Sleep(30 * time.Millisecond)
	}
	cancelledEvents := jsonResponse[[]object](t, r, "GET", "/runs/"+cancelledID+"/events", "", 200)
	if len(cancelledEvents) < 3 || cancelledEvents[len(cancelledEvents)-2]["summary"] != "已核对资料。" {
		t.Fatal(cancelledEvents)
	}
	explicit, err := http.NewRequest("POST", api.URL+apiPrefix+"/agent/runs",
		strings.NewReader(`{"intent":"停止测试","model":"folio-agent"}`))
	if err != nil {
		t.Fatal(err)
	}
	explicit.Header.Set("Authorization", "Bearer "+operatorToken)
	explicit.Header.Set("Content-Type", "application/json")
	client := api.Client()
	client.Timeout = 10 * time.Second
	response, err = client.Do(explicit)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	reader = bufio.NewScanner(response.Body)
	var explicitID string
	for reader.Scan() {
		var event object
		if data, ok := strings.CutPrefix(reader.Text(), "data: "); ok && json.Unmarshal([]byte(data), &event) == nil {
			if event["type"] == "run_started" {
				explicitID = event["run_id"].(string)
			}
			if event["type"] == "token" {
				break
			}
		}
	}
	if explicitID == "" {
		t.Fatal("no run ID for explicit cancel")
	}
	cancelPath := "/agent/runs/" + explicitID + "/cancel"
	for range 2 {
		stopped := jsonResponse[object](t, r, "POST", cancelPath, "", 200)
		if stopped["status"] != "cancelled" {
			t.Fatal(stopped)
		}
	}
	var terminal object
	for reader.Scan() {
		if data, ok := strings.CutPrefix(reader.Text(), "data: "); ok {
			if err := json.Unmarshal([]byte(data), &terminal); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := reader.Err(); err != nil || terminal["type"] != "done" || terminal["status"] != "cancelled" {
		t.Fatalf("cancel terminal=%v err=%v", terminal, err)
	}
	select {
	case <-upstreamStopped:
	case <-time.After(5 * time.Second):
		t.Fatal("explicit cancel did not stop Agent")
	}

}
