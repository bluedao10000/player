package knowledge

import (
	"context"
	"testing"

	"anime-agent/api/internal/testsupport"
	sqltest "anime-agent/database/queries/fixtures"
)

func TestEditorialProfilesShareAWorkHead(t *testing.T) {
	ctx := context.Background()
	db := testsupport.Database(t)
	if err := Sweep(ctx, db, "tidepost-original", true, false); err != nil {
		t.Fatal(err)
	}
	const world = "tidepost"
	const chapter = "night-courier"
	var snapshot editSnapshot
	if err := db.QueryRow(ctx, sqltest.ProfileSnapshot, world, chapter).Scan(&snapshot); err != nil {
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
		err := db.QueryRow(ctx, sqltest.CreateProfileReview, world, chapter, draft).Scan(&id)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	current := func() (string, string) {
		t.Helper()
		var b, s string
		if err := db.QueryRow(ctx, sqltest.PublishedProfiles, world).Scan(&b, &s); err != nil {
			t.Fatal(err)
		}
		return b, s
	}
	first, stale := create(both), create(both)
	if b, s := current(); b != fields["person/mio"] || s != "" {
		t.Fatal("draft is public")
	}
	var preview string
	if err := db.QueryRow(ctx, sqltest.PreviewWorldSetting, first).Scan(&preview); err != nil || preview != setting {
		t.Fatal("missing world preview", err)
	}
	approve := func(id string) error { return ReviewEdit(ctx, db, id, "approve", "tester", "", 1, 1) }
	if err := approve(first); err != nil {
		t.Fatal(err)
	}
	if err := approve(stale); err != ErrEditStale {
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
	if err := db.QueryRow(ctx, sqltest.CountChapterHeads, world).Scan(&count); err != nil || count != 0 {
		t.Fatal("chapter publication was affected", err)
	}
}
