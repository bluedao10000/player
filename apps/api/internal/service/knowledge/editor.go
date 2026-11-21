package knowledge

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	queries "anime-agent/database/queries/knowledge"
)

type editBlock struct{ Key, Text string }
type editSnapshot struct {
	ProfileFields map[string]string `json:"fields"`
	ResearchRun   string            `json:"research_run"`
	Chapter       struct {
		Blocks   []editBlock
		Passages []editBlock `json:"reader_passages"`
		Research struct {
			Run string `json:"run_id"`
		}
	}
	Scenes map[string]struct{ Scene struct{ Text string } }
}

func (s editSnapshot) fields() map[string]string {
	if s.ProfileFields != nil {
		return s.ProfileFields
	}
	out := map[string]string{}
	blocks := s.Chapter.Passages
	if blocks == nil {
		blocks = s.Chapter.Blocks
	}
	for _, b := range blocks {
		out["block/"+b.Key] = b.Text
	}
	for k, v := range s.Scenes {
		out["scene/"+k] = v.Scene.Text
	}
	return out
}

type editSource struct {
	ID      string `json:"id"`
	URI     string `json:"uri"`
	Locator string `json:"locator"`
	Text    string `json:"text"`
	Digest  string `json:"sha256"`
}
type editChange struct {
	Key    string `json:"key"`
	Text   string `json:"text"`
	Source string `json:"source"`
	Quote  string `json:"quote"`
}
type copyField struct {
	Original string `json:"original"`
	Text     string `json:"text"`
}
type editorJob struct {
	ID, World, Chapter, Instruction, Feedback, Issue, Status, Lease string
	Round, Attempt, Retries                                         int
	Base                                                            editSnapshot
	Sources                                                         []editSource
	Draft                                                           []editChange
}

func RecordEditorEvent(ctx context.Context, tx pgx.Tx, id, stage, note string, detail any) error {
	_, err := tx.Exec(ctx, queries.RecordEditorEvent, id, stage, note, detail)
	return err
}

var ErrEditStale = errors.New("此稿已变化，请刷新并核对最新版本")

func ReviewEdit(ctx context.Context, db *pgxpool.Pool, id, action, operator, note string, round, attempt int) error {
	stale := false
	err := pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		var world, chapter, status, kind string
		var r, a int
		var draft []editChange
		var base editSnapshot
		err := tx.QueryRow(ctx, queries.LockEditorTask, id).Scan(&world, &chapter, &kind, &status, &r, &a, &draft, &base)
		if err != nil {
			return err
		}
		if r != round || a != attempt {
			return ErrEditStale
		}
		if action == "return" {
			if status != "review" && status != "failed" {
				return ErrEditStale
			}
			if err = RecordEditorEvent(ctx, tx, id, "returned", operator+" · "+note, draft); err != nil {
				return err
			}
			_, err = tx.Exec(ctx, queries.ReturnEditorTask, id, note)
			return err
		}
		if status == "published" {
			return nil
		}
		if status != "review" {
			return ErrEditStale
		}
		var headChapter *string
		lockKey := world + "/profiles"
		if kind == "chapter" {
			headChapter = &chapter
			lockKey = world + "/chapter/" + chapter
		}
		if _, err = tx.Exec(ctx, queries.LockEditorHead, lockKey); err != nil {
			return err
		}
		var same bool
		var patch map[string]copyField
		err = tx.QueryRow(ctx, queries.CompareEditBase, id, headChapter).Scan(&same, &patch)
		if err != nil {
			return err
		}
		if !same {
			stale = true
			if _, err = tx.Exec(ctx, queries.MarkEditConflict, id); err != nil {
				return err
			}
			return RecordEditorEvent(ctx, tx, id, "conflict", "公开定稿已变化，未覆盖新版本", nil)
		}
		fields := base.fields()
		for _, d := range draft {
			before := fields[d.Key]
			original := before
			if old, ok := patch[d.Key]; ok && old.Text == before {
				original = old.Original
			}
			patch[d.Key] = copyField{original, d.Text}
		}
		if _, err = tx.Exec(ctx, queries.PublishEditorHead, world, headChapter, id, patch); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, queries.MarkEditPublished, id, operator); err != nil {
			return err
		}
		return RecordEditorEvent(ctx, tx, id, "published", operator+" 审核通过，整项发布", draft)
	})
	if err != nil {
		return err
	}
	if stale {
		return ErrEditStale
	}
	return nil
}

func CreateEdit(ctx context.Context, db *pgxpool.Pool, requestID, world, chapter, kind, instruction, operator string) (string, error) {
	var id string
	err := pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, queries.CreateEditorTask, requestID, world, chapter, instruction, operator, kind).Scan(&id)
		if errors.Is(err, pgx.ErrNoRows) {
			return tx.QueryRow(ctx, queries.FindDuplicateTask, requestID, world, chapter, instruction, operator, kind).Scan(&id)
		}
		if err == nil {
			return RecordEditorEvent(ctx, tx, id, "created", "已保存编辑意见与对应页面的公开底稿", nil)
		}
		return err
	})
	return id, err
}
