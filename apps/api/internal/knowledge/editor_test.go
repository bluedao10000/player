package knowledge

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestEditorialReviewAndPublication(t *testing.T) {
	ctx := context.Background()
	db := testStore(t)
	if err := Sweep(ctx, db, "tidepost-original", true, false); err != nil {
		t.Fatal(err)
	}
	admin := strings.Repeat("t", 40)
	t.Setenv("KNOWLEDGE_ADMIN_TOKEN", admin)
	t.Setenv("KNOWLEDGE_ADMIN_OPERATOR", "test-editor")
	r := gin.New()
	Routes(r, db)
	call := func(method, path, token string, payload any, want int) []byte {
		t.Helper()
		body, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(method, "/api/v1/"+path, bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != want {
			t.Fatalf("%s %s: %d: %s", method, path, w.Code, w.Body.String())
		}
		return w.Body.Bytes()
	}
	obj := func(raw []byte) map[string]string {
		t.Helper()
		var v map[string]string
		if err := json.Unmarshal(raw, &v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	session := obj(call("POST", "admin/knowledge/editor/session", admin, nil, 200))["token"]
	create := func(key string) string {
		return obj(call("POST", "editor/tasks", session, gin.H{"world": "tidepost", "chapter": "night-courier", "instruction": "修订邮筒发现的正文和场景", "request_id": key}, 200))["id"]
	}
	id := create("98b5fb50-a85e-48ec-a00e-746c2cdd45c2")
	if create("98b5fb50-a85e-48ec-a00e-746c2cdd45c2") != id {
		t.Fatal("duplicate submission")
	}
	call("POST", "editor/tasks", session, gin.H{"world": "other", "chapter": "night-courier", "instruction": "补充", "request_id": "beeb2d2a-4ea6-4eed-b7c8-a5a19c097f9b"}, 400)
	second := create("8c61ec83-fa29-444c-87bf-0b907ea87964")
	task := "editor/tasks/" + id
	decision := func(action string, round, attempt int, note string, code int) {
		call("POST", task+"/"+action, session, gin.H{"round": round, "attempt": attempt, "note": note}, code)
	}
	decision("approve", 1, 1, "", 409)
	chapter := "knowledge/worlds/tidepost/chapters/night-courier"
	scene := "knowledge/worlds/tidepost/scenes/unstamped-letter"
	initialCh := call("GET", chapter, "", nil, 200)
	initialScene := call("GET", scene, "", nil, 200)
	const revision = "阿澪在红邮筒里找到了一封信。"
	model := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		var input struct{ Sources []editSource }
		if err := json.NewDecoder(req.Body).Decode(&input); err != nil || len(input.Sources) == 0 {
			http.Error(w, "no evidence", 400)
			return
		}
		s := input.Sources[0]
		if strings.HasSuffix(req.URL.Path, "/drafting") {
			json.NewEncoder(w).Encode([]editChange{{"block/mailbox", revision, s.ID, s.Text}, {"scene/unstamped-letter", revision, s.ID, s.Text}})
		} else if strings.Contains(req.URL.Path, "/1/1/") {
			json.NewEncoder(w).Encode(gin.H{"pass": false, "note": "缩短场景说明"})
		} else {
			json.NewEncoder(w).Encode(gin.H{"pass": true, "note": "已核对原创定稿"})
		}
	}))
	defer model.Close()
	drain := func() {
		t.Helper()
		for i := 0; i < 32; i++ {
			var count int
			if err := db.QueryRow(ctx, "SELECT count(*) FROM editor_tasks WHERE status IN ('evidence','drafting','checking')").Scan(&count); err != nil {
				t.Fatal(err)
			}
			if count == 0 {
				return
			}
			if err := ProcessEditorial(ctx, db, model.URL); err != nil {
				t.Fatal(err)
			}
		}
		t.Fatal("editor did not settle")
	}
	drain()
	call("GET", task+"/preview?round=1&attempt=2", "", nil, 401)
	if !bytes.Equal(initialCh, call("GET", chapter, "", nil, 200)) || !bytes.Equal(initialScene, call("GET", scene, "", nil, 200)) {
		t.Fatal("review changed public pages")
	}
	if !bytes.Contains(call("GET", task+"/preview?round=1&attempt=2", session, nil, 200), []byte(revision)) {
		t.Fatal("preview missing updated copy")
	}
	decision("return", 1, 2, "再精简一些", 204)
	call("GET", task+"/preview?round=1&attempt=2", session, nil, 404)
	drain()
	decision("approve", 1, 2, "", 409)
	decision("approve", 2, 1, "", 204)
	decision("approve", 2, 1, "", 204)
	currentCh := call("GET", chapter, "", nil, 200)
	currentScene := call("GET", scene, "", nil, 200)
	if !bytes.Contains(currentCh, []byte(revision)) || !bytes.Contains(currentScene, []byte(revision)) || !bytes.Contains(currentCh, []byte("passage-unstamped-letter")) || !bytes.Contains(currentScene, []byte("/story/night-courier#passage-unstamped-letter")) {
		t.Fatal("release lost text or anchor")
	}
	task = "editor/tasks/" + second
	decision("approve", 1, 2, "", 409)
	var status string
	if err := db.QueryRow(ctx, "select status from editor_tasks where id=$1", second).Scan(&status); err != nil || status != "conflict" {
		t.Fatal("stale task overwrote release", status, err)
	}
	call("POST", "editor/logout", session, nil, 204)
	call("GET", "editor/tasks", session, nil, 401)
}

func TestEditorialProfilesShareAWorkHead(t *testing.T) {
	ctx := context.Background()
	db := testStore(t)
	if err := Sweep(ctx, db, "tidepost-original", true, false); err != nil {
		t.Fatal(err)
	}
	const world = "tidepost"
	const chapter = "night-courier"
	var snapshot editSnapshot
	if err := db.QueryRow(ctx, "select editor_task_snapshot($1,$2,'profiles')", world, chapter).Scan(&snapshot); err != nil {
		t.Fatal(err)
	}
	fields := snapshot.fields()
	if _, ok := fields["person/captain"]; ok || len(fields) != 2 || fields["person/mio"] == "" || fields["world/setting"] != "" {
		t.Fatalf("incorrect chapter scope: %v", fields)
	}
	sources := []editSource{{ID: "src", Text: "阿澪打开了红邮筒。"}}
	job := editorJob{Base: snapshot, Sources: sources}
	if err := checkChanges(job, []editChange{{"person/captain", "船长出现", "src", "阿澪打开了红邮筒"}}); err == nil {
		t.Fatal("unrelated character admitted")
	}
	const bio = "阿澪上夜班时先打开红邮筒。"
	const setting = "潮水漫过白帆港栈桥时，末班船正在解缆。"
	both := []editChange{{"person/mio", bio, "src", "阿澪打开了红邮筒"}, {"world/setting", setting, "src", "阿澪打开了红邮筒"}}
	if err := checkChanges(job, both); err != nil {
		t.Fatal(err)
	}
	create := func(draft []editChange) string {
		t.Helper()
		var id string
		err := db.QueryRow(ctx, `INSERT INTO editor_tasks(request_id,world,chapter,kind,instruction,author,base,base_head,status,draft)
 SELECT gen_random_uuid(),$1,$2,'profiles','修改介绍','tester',editor_task_snapshot($1,$2,'profiles'),(SELECT task FROM editor_heads WHERE world=$1 AND chapter IS NULL),'review',$3 RETURNING id`, world, chapter, draft).Scan(&id)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	current := func() (string, string) {
		t.Helper()
		var b, s string
		if err := db.QueryRow(ctx, `SELECT editor_profile_text(editor_public_profile(editor_raw_person($1,'mio'),$1,'person/mio'),'person/mio'),
 editor_profile_text(editor_public_profile(editor_raw_overview($1),$1,'world/setting'),'world/setting')`, world).Scan(&b, &s); err != nil {
			t.Fatal(err)
		}
		return b, s
	}
	first, stale := create(both), create(both)
	if b, s := current(); b != fields["person/mio"] || s != "" {
		t.Fatal("draft is public")
	}
	var preview string
	if err := db.QueryRow(ctx, `select (editor_profile_preview(base,draft)#>>'{profile_pages,world/setting,world_setting}') from editor_tasks where id=$1`, first).Scan(&preview); err != nil || preview != setting {
		t.Fatal("missing world preview", err)
	}
	approve := func(id string) error { return editorDecision(ctx, db, id, "approve", "tester", "", 1, 1) }
	if err := approve(first); err != nil {
		t.Fatal(err)
	}
	if err := approve(stale); err != errEditStale {
		t.Fatalf("stale publish: %v", err)
	}
	if b, s := current(); b != bio || s != setting {
		t.Fatal("partial publication", b, s)
	}
	both[1].Text = "白帆港的末班船正在解缆。"
	if err := approve(create(both[1:])); err != nil {
		t.Fatal(err)
	}
	if b, s := current(); b != bio || s != both[1].Text {
		t.Fatal("world-only edit lost previous bio", b, s)
	}
	var count int
	if err := db.QueryRow(ctx, "SELECT count(*) FROM editor_heads WHERE world=$1 AND chapter IS NOT NULL", world).Scan(&count); err != nil || count != 0 {
		t.Fatal("chapter publication was affected", err)
	}
}
