package knowledge

import (
	"context"
	"encoding/json"
	"testing"

	"anime-agent/api/internal/testsupport"
	sqltest "anime-agent/api/internal/testsupport/fixtures"
)

// Reframing a valid original keeps the original version. An unfixable defect
// gets one new original; a second rejection is held with its review reason.
func TestMediaReworkBudget(t *testing.T) {
	db := testsupport.Database(t)
	ctx := context.Background()
	pc, err := db.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer pc.Release()
	if _, err = pc.Exec(ctx, sqltest.SetReviewStage); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		step      string
		round     int
		result    string
		next      string
		nextRound int
		focus     int
	}{
		{"review", 1, `{"decision":"recrop","reason":"保留帽檐","focus":10}`, "recrop", 1, 10},
		{"recrop", 1, `{"phone":"mio-1-r1-crop1.jpg","focus":10}`, "recheck", 1, 10},
		{"recheck", 1, `{"decision":"redraw","reason":"手部遮挡信件"}`, "generate", 2, 40},
		{"generate", 2, `{"image":"mio-1-r2.png","phone":"mio-1-r2-crop0.jpg","sha":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef","reference_sha":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}`, "review", 2, 40},
		{"review", 2, `{"decision":"redraw","reason":"手部仍有异常"}`, "held", 2, 40},
	} {
		var j mediaJob
		if err = pc.QueryRow(ctx, sqltest.CandidateAttempt).Scan(&j.Attempt, &j.Slot, &j.Round, &j.Step, &j.Focus, &j.Reason); err != nil {
			t.Fatal(err)
		}
		if j.Round != tc.round || j.Step != tc.step {
			t.Fatal(j)
		}
		if _, err = pc.Exec(ctx, sqltest.InsertReviewStep, j.Round, j.Step); err != nil {
			t.Fatal(err)
		}
		if err = advanceMedia(ctx, pc.Conn(), j, mediaReply{State: "done", Result: json.RawMessage(tc.result)}); err != nil {
			t.Fatal(err)
		}
		var stage string
		var round, focus int
		if err = pc.QueryRow(ctx, sqltest.CandidateFocus).Scan(&stage, &round, &focus); err != nil || stage != tc.next || round != tc.nextRound || focus != tc.focus {
			t.Fatalf("%s -> %s #%d @%d: %v", j.Step, stage, round, focus, err)
		}
	}
}

func TestMediaBatchReference(t *testing.T) {
	db := testsupport.Database(t)
	ctx := context.Background()
	pc, err := db.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer pc.Release()
	var j mediaJob
	err = pc.QueryRow(ctx, sqltest.BebopReferenceCandidate).Scan(&j.Slot, &j.Position, &j.Batch, &j.Step, &j.Round, &j.Attempt, &j.Focus)
	if err != nil || j.Slot == j.Position || j.Step != "reference" {
		t.Fatalf("batch identity: %+v %v", j, err)
	}
	if _, err = pc.Exec(ctx, sqltest.InsertReferenceStep, j.Slot); err != nil {
		t.Fatal(err)
	}
	const sha = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	reply := mediaReply{State: "done", Result: json.RawMessage(`{"reference_sha":"` + sha + `"}`)}
	if err = advanceMedia(ctx, pc.Conn(), j, reply); err != nil {
		t.Fatal(err)
	}
	var stage string
	if err = pc.QueryRow(ctx, sqltest.CandidateReference, j.Slot).Scan(&stage, &j.ReferenceSHA); err != nil || stage != "generate" || j.ReferenceSHA != sha {
		t.Fatal(stage, j.ReferenceSHA, err)
	}
	if err = pc.QueryRow(ctx, sqltest.MioStage).Scan(&stage); err != nil || stage != "queued" {
		t.Fatal("other batch changed", stage, err)
	}
	j.Step = "generate"
	if advanceMedia(ctx, pc.Conn(), j, mediaReply{State: "done", Result: json.RawMessage(`{"image":"bebop-1-r1.png","phone":"bebop-1-r1-crop0.jpg","sha":"` + sha + `","reference_sha":"0000000000000000000000000000000000000000000000000000000000000000"}`)}) == nil {
		t.Fatal("wrong reference digest accepted")
	}
}

func TestPortraitPublicationRequiresPassingChecksForRenderedVersion(t *testing.T) {
	db := testsupport.Database(t)
	ctx := context.Background()
	pc, err := db.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer pc.Release()
	const digest = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	j := mediaJob{Batch: "bebop-portrait", Position: 1, Slot: 19, Round: 1, Attempt: 1, Step: "review", ReferenceSHA: digest}
	_, err = pc.Exec(ctx, sqltest.PrepareJetPortrait, digest)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pc.Exec(ctx, sqltest.InsertPortraitRender, digest)
	if err != nil {
		t.Fatal(err)
	}
	verdict := func(phone string, identity bool) error {
		result, _ := json.Marshal(map[string]any{"decision": "pass", "phone": phone, "checks": map[string]bool{
			"identity": identity, "scene": true, "anatomy": true, "clean": true, "crop": true,
		}})
		return advanceMedia(ctx, pc.Conn(), j, mediaReply{State: "done", Result: result})
	}
	if verdict("wrong-round.jpg", true) == nil || verdict("crop.jpg", false) == nil {
		t.Fatal("incomplete or mismatched evidence accepted")
	}
	var count int
	if err = db.QueryRow(ctx, sqltest.CountJetPortraits).Scan(&count); err != nil || count != 0 {
		t.Fatalf("premature publication: %d %v", count, err)
	}
	if err = verdict("crop.jpg", true); err != nil {
		t.Fatal(err)
	}
	var path, stage string
	if err = db.QueryRow(ctx, sqltest.PublishedPortrait).Scan(&path, &stage); err != nil || path != "/art/portraits/"+digest+".webp" || stage != "passed" {
		t.Fatalf("publication mismatch: %q %q %v", path, stage, err)
	}
}
