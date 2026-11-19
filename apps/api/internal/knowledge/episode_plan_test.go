package knowledge

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Reuse one registered edition and its permanent routes even if the worker
// polls twice or an operator appends another episode to the mounted manifest.
func TestAppendEpisodeConfiguration(t *testing.T) {
	db := testStore(t)
	ctx := context.Background()
	raw, err := os.ReadFile("../../../../data/episode-plans/naruto-tv-2002.json")
	if err != nil {
		t.Fatal(err)
	}
	var w editionPlan
	if err = json.Unmarshal(raw, &w); err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(ctx, `WITH k AS (INSERT INTO works(canonical_title,type,season_year) VALUES('fixture','TV',2002) RETURNING id) INSERT INTO work_sources SELECT id,'bangumi','3425','https://bgm.tv/subject/3425' FROM k`)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	t.Setenv("EPISODE_PLAN_DIR", dir)
	write := func() {
		t.Helper()
		b, e := json.Marshal(w)
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(filepath.Join(dir, "edition.json"), b, 0600); e != nil {
			t.Fatal(e)
		}
	}
	load := func() {
		t.Helper()
		if e := LoadEpisodePlans(ctx, db); e != nil {
			t.Fatal(e)
		}
	}
	signature := func() string {
		t.Helper()
		var ids string
		if err := db.QueryRow(ctx, "SELECT string_agg(id||':'||(spec->>'slug'),',' ORDER BY number) FROM episode_plans WHERE number<=5").Scan(&ids); err != nil {
			t.Fatal(err)
		}
		return ids
	}
	write()
	load()
	before := signature()
	load()
	var extra map[string]any
	if err = json.Unmarshal(w.Episodes[4], &extra); err != nil {
		t.Fatal(err)
	}
	extra["number"] = 6
	extra["slug"] = "new-chapter"
	extra["sceneSlug"] = "new-scene"
	extra["guide"] = "https://en.wikipedia.org/wiki/Naruto_season_1#ep6"
	b, err := json.Marshal(extra)
	if err != nil {
		t.Fatal(err)
	}
	w.Episodes = append(w.Episodes, b)
	write()
	load()
	var topics, art, versions int
	err = db.QueryRow(ctx, "SELECT (SELECT count(*) FROM episode_plans),(SELECT count(*) FROM episode_art_jobs),(SELECT count(*) FROM world_releases)").Scan(&topics, &art, &versions)
	if err != nil || topics != 6 || art != 12 || versions != 1 || signature() != before {
		t.Fatalf("append: topics=%d jobs=%d releases=%d err=%v", topics, art, versions, err)
	}
	extra["slug"] = "changed-route"
	w.Episodes[5], _ = json.Marshal(extra)
	write()
	if LoadEpisodePlans(ctx, db) == nil {
		t.Fatal("accepted a mutated registered route")
	}
	if err = AcceptPlannedEpisodes(ctx, db); err != nil {
		t.Fatal(err)
	}
	var heads, public int
	err = db.QueryRow(ctx, "SELECT count(*),count(published) FROM scope_heads").Scan(&heads, &public)
	if err != nil || heads != 6 || public != 0 {
		t.Fatalf("pending acceptance heads=%d published=%d err=%v", heads, public, err)
	}
}

func TestCompleteONARequiresEveryEpisode(t *testing.T) {
	db := testStore(t)
	dir := t.TempDir()
	t.Setenv("EPISODE_PLAN_DIR", dir)
	ctx := context.Background()
	raw, err := os.ReadFile("../../../../data/episode-plans/takopi-ona-2025.json")
	if err != nil {
		t.Fatal(err)
	}
	var plan editionPlan
	if err = json.Unmarshal(raw, &plan); err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(ctx, `WITH k AS (INSERT INTO works(canonical_title,type,season_year,status,episodes) VALUES('fixture','ONA',2025,'FINISHED',6) RETURNING id) INSERT INTO work_sources SELECT id,'bangumi','527620','https://bgm.tv/subject/527620' FROM k`)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "whole.json")
	put := func(b []byte) {
		t.Helper()
		if err := os.WriteFile(path, b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	missing := plan
	missing.Episodes = missing.Episodes[:5]
	gap, err := json.Marshal(missing)
	if err != nil {
		t.Fatal(err)
	}
	put(gap)
	if LoadEpisodePlans(ctx, db) == nil {
		t.Fatal("accepted partial manifest as complete")
	}
	put(raw)
	if err = LoadEpisodePlans(ctx, db); err != nil {
		t.Fatal(err)
	}
	if err = LoadEpisodePlans(ctx, db); err != nil {
		t.Fatal(err)
	}
	var episodes, published int
	err = db.QueryRow(ctx, `SELECT (SELECT count(*) FROM episode_plans WHERE world_slug=$1),(SELECT count(*) FROM published_episode_scopes WHERE world_slug=$1)`, plan.World).Scan(&episodes, &published)
	if err != nil || episodes != 6 || published != 0 {
		t.Fatalf("episodes=%d published=%d err=%v", episodes, published, err)
	}
	// Two failed HTTP dispatches exhaust the normal automatic retries. Repairing
	// the upstream should reopen only the failed generation, preserving both receipts.
	if _, err = db.Exec(ctx, `UPDATE media_candidates SET art_job=(SELECT min(id) FROM episode_art_jobs),stage='failed',reason='模型服务 HTTP 403' WHERE slot IN (1,2);
		INSERT INTO media_steps(slot,round,step,attempt,error,finished_at) VALUES
		(1,1,'generate',1,'模型服务 HTTP 502',now()),(1,1,'generate',2,'模型服务 HTTP 403',now()),
		(2,1,'review',1,'模型服务 HTTP 403',now())`); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []int64{1, 0} {
		n, err := ResumeImageProviderFailures(ctx, db, plan.World)
		if err != nil || n != expected {
			t.Fatalf("resumed %d expected %d: %v", n, expected, err)
		}
	}
	var attempt, receipts int
	if err = db.QueryRow(ctx, `SELECT attempt,(SELECT count(*) FROM media_steps WHERE slot=1) FROM media_candidates WHERE slot=1 AND stage='generate'`).Scan(&attempt, &receipts); err != nil || attempt != 3 || receipts != 2 {
		t.Fatalf("attempt=%d receipts=%d err=%v", attempt, receipts, err)
	}
}
