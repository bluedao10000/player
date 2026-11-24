package chattools

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestSnapshotV1(t *testing.T) {
	facts, names, err := Snapshot(json.RawMessage(`{
   "work":{"slug":"fixture"},
   "inventory":{"checked_at":"2026-11-24T08:00:00+08:00"},
   "editor_task":null,
   "governance":{"open_quality_issues":1},
   "completeness":[{"dimension":"people"}],
   "plan":{"revision":2},
   "episodes":[{"number":3}],
   "media":[{"slot":4}],
   "history":[{"topic":"fixture-3","stage":"ready"}]
 }`))
	if err != nil {
		t.Fatal(err)
	}
	wantNames := []string{
		"get_workspace_context", "get_knowledge_completeness", "get_production_plan",
		"get_episode_status", "get_media_candidates", "get_execution_history",
	}
	if !reflect.DeepEqual(names, wantNames) {
		t.Fatalf("v1 names changed: %v", names)
	}
	got, err := json.Marshal(facts)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"get_episode_status":[{"number":3}],"get_execution_history":[{"topic":"fixture-3","stage":"ready"}],"get_knowledge_completeness":[{"dimension":"people"}],"get_media_candidates":[{"slot":4}],"get_production_plan":{"revision":2},"get_workspace_context":{"editor_task":null,"governance":{"open_quality_issues":1},"inventory":{"checked_at":"2026-11-24T08:00:00+08:00"},"work":{"slug":"fixture"}}}`
	if string(got) != want {
		t.Fatalf("v1 snapshot changed: %s", got)
	}
}
