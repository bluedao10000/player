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
