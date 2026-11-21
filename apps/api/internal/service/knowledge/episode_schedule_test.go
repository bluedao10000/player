package knowledge

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"testing"

	"anime-agent/api/internal/testsupport"
	sqltest "anime-agent/api/internal/testsupport/fixtures"
)

func TestProductionAdmission(t *testing.T) {
	db := testsupport.Database(t)
	ctx := context.Background()
	raw, err := os.ReadFile("testdata/cowboy-bebop-tv.json")
	if err != nil {
		t.Fatal(err)
	}
	var w editionPlan
	if err = json.Unmarshal(raw, &w); err != nil {
		t.Fatal(err)
	}
	// Fixture uses a distinct edition and a cast card without a prepublished portrait.
	w.World = "schedule-fixture"
	w.Provider = "fixture"
	w.ExternalID = "1"
	w.People = w.People[:1]
	w.People[0].ReusePortrait = false
	var spec map[string]any
	if err = json.Unmarshal(w.Episodes[0], &spec); err != nil {
		t.Fatal(err)
	}
	w.People[0].Portrait, _ = json.Marshal(spec["art"])
	spec["people"] = []map[string]any{{"slug": w.People[0].Slug, "name": w.People[0].Name, "beat": 1}}
	w.Episodes[0], _ = json.Marshal(spec)
	w.Automation.Last = 6
	spec["number"] = 6
	spec["slug"] = "fixture-next"
	spec["sceneSlug"] = "fixture-next"
	spec["guide"] = "https://en.wikipedia.org/wiki/List_of_Cowboy_Bebop_episodes#ep6"
	next, _ := json.Marshal(spec)
	w.Episodes = append(w.Episodes, next)
	if _, err = db.Exec(ctx, sqltest.CreateScheduleEdition); err != nil {
		t.Fatal(err)
	}
	register := func() {
		t.Helper()
		raw, _ := json.Marshal(w)
		if err := RegisterProductionPlan(ctx, db, bytes.NewReader(raw)); err != nil {
			t.Fatal(err)
		}
	}
	register()
	register()
	admit := func(want int) {
		t.Helper()
		if err := AdmitEpisode(ctx, db); err != nil {
			t.Fatal(err)
		}
		var n int
		if err := db.QueryRow(ctx, sqltest.CountAdmissions).Scan(&n); err != nil || n != want {
			t.Fatalf("admitted %d want %d: %v", n, want, err)
		}
	}
	admit(0)
	if _, err = db.Exec(ctx, sqltest.InsertMissingEpisodes); err != nil {
		t.Fatal(err)
	}
	admit(1)
	admit(1)
	w.Automation.MaxEpisodes = 2
	register()
	admit(1)
	var status string
	if err = db.QueryRow(ctx, sqltest.QueueStatus).Scan(&status); err != nil || status != "token 预算不足" {
		t.Fatalf("%s %v", status, err)
	}
	var job bytes.Buffer
	if err := ClaimCapture(ctx, db, &job); err != nil {
		t.Fatal(err)
	}
	var j map[string]any
	if err = json.Unmarshal(job.Bytes(), &j); err != nil {
		t.Fatal(err)
	}
	j["error"] = "fixture source unavailable"
	payload, _ := json.Marshal(j)
	if err = ImportEpisodeCapture(ctx, db, bytes.NewReader(payload)); err != nil {
		t.Fatal(err)
	}
	if err = ImportEpisodeCapture(ctx, db, bytes.NewReader(payload)); err == nil {
		t.Fatal("accepted duplicate capture lease")
	}
}
