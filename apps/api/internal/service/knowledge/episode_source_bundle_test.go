package knowledge

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	queries "anime-agent/api/internal/repository/queries/knowledge"
	"anime-agent/api/internal/testsupport"
	sqltest "anime-agent/api/internal/testsupport/fixtures"
	"github.com/jackc/pgx/v5"
)

func TestAttachEpisodeSourceBundle(t *testing.T) {
	db := testsupport.Database(t)
	ctx := context.Background()
	if _, err := db.Exec(ctx, sqltest.CreateNarutoEdition); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("testdata/naruto-tv-2002.json")
	if err != nil {
		t.Fatal(err)
	}
	var edition editionPlan
	if err = json.Unmarshal(raw, &edition); err != nil {
		t.Fatal(err)
	}
	if err = pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error { return registerEdition(ctx, tx, edition) }); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, sqltest.SeedBlockedSourceBundleRequests); err != nil {
		t.Fatal(err)
	}
	rules := `{"version":1,"guide":"paired","officialRule":{"kind":"json_episode_list","episode":"episode","title":"title","body":"text"},"guideRule":{"kind":"html_episode_article","heading":"h1","body":"article > p"}}`
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(rules)))
	proof := fmt.Sprintf(`[{"episode":6,"pageSha256":%q,"textSha256":%q},{"episode":7,"pageSha256":%q,"textSha256":%q}]`, digest, digest, digest, digest)
	verified := fmt.Sprintf(`{"sourceRules":%s,"ruleSha256":%q,"sources":[
{"number":6,"title":"Episode Six","official":"https://naruto-official.com/en/anime/","guide":"https://skypenguin.net/fixture-6/"},
{"number":7,"title":"Episode Seven","official":"https://naruto-official.com/en/anime/","guide":"https://skypenguin.net/fixture-7/"}],
"proofs":{"official":%s,"guide":%s}}`, rules, digest, proof, proof)
	for _, want := range []int64{2, 0} {
		got, err := AttachEpisodeSourceBundle(ctx, db, "naruto-tv-2002", strings.NewReader(verified))
		if err != nil || got != want {
			t.Fatalf("updated %d, want %d: %v", got, want, err)
		}
	}
	var count int
	if err := db.QueryRow(ctx, sqltest.AttachedSourceBundleCount, digest).Scan(&count); err != nil || count != 2 {
		t.Fatalf("attached %d: %v", count, err)
	}
	if _, err := AttachEpisodeSourceBundle(ctx, db, "naruto-tv-2002", strings.NewReader(strings.Replace(verified, "article > p", "body", 1))); err == nil {
		t.Fatal("accepted a source rule changed after validation")
	}
	if _, err := db.Exec(ctx, sqltest.MoveSourceBundleToConfiguration); err != nil {
		t.Fatal(err)
	}
	var id, world, brief string
	var revision, number int
	var template []byte
	if err := db.QueryRow(ctx, queries.Configuration["Next"]).Scan(&id, &revision, &world, &number, &brief, &template); err != nil {
		t.Fatal(err)
	}
	var input struct {
		SourceBundle struct {
			Number     int
			RuleSHA256 string
		}
	}
	if err := json.Unmarshal(template, &input); err != nil || number != 6 || input.SourceBundle.Number != 6 || input.SourceBundle.RuleSHA256 != digest {
		t.Fatalf("configuration received wrong source bundle: number=%d err=%v", number, err)
	}
}
