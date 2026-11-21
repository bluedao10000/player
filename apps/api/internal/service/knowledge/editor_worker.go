package knowledge

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	queries "anime-agent/api/internal/repository/queries/knowledge"
)

func editorialSources(ctx context.Context, db *pgxpool.Pool, j editorJob, endpoint string) ([]editSource, error) {
	var result []editSource
	if j.World == "tidepost" {
		raw, e := worldFiles.ReadFile("worlddata/tidepost.json")
		if e != nil {
			return nil, e
		}
		var m worldManifest
		if e = json.Unmarshal(raw, &m); e != nil {
			return nil, e
		}
		add := func(id, locator, text string) {
			result = append(result, editSource{id, "local:worlddata/tidepost.json", locator, text, fmt.Sprintf("%x", sha256.Sum256([]byte(text)))})
		}
		if j.Base.ProfileFields != nil {
			add("work-intro", m.Name+" / 作品简介", m.Intro)
			for _, p := range m.People {
				if _, ok := j.Base.ProfileFields["person/"+p.Slug]; ok {
					add("cast-"+p.Slug, "人物设定 / "+p.Name, p.Name+" · "+p.Role)
				}
			}
		}
		for _, ch := range m.Chapters {
			if ch.Slug != j.Chapter {
				continue
			}
			for _, b := range ch.Blocks {
				add("canon-"+b.Key, ch.Name+" / "+b.Key, b.Text)
			}
			for _, s := range ch.Scenes {
				add("scene-"+s.Slug, s.Name, s.Text)
			}
		}
	} else {
		var docs []struct {
			ID, URL, Locator, Text, SHA256 string
			Episode                        int
			Fetched                        string `json:"fetched_at"`
		}
		var plan json.RawMessage
		if err := db.QueryRow(ctx, queries.EditorialEpisodeSources, j.World).Scan(&plan); err != nil {
			return nil, err
		}
		liveCtx, cancel := context.WithTimeout(ctx, 32*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(liveCtx, "POST", strings.TrimRight(endpoint, "/")+"/internal/editor/evidence/naruto-1", bytes.NewReader(plan))
		if err != nil {
			return nil, err
		}
		resp, err := http.DefaultClient.Do(req)
		live := err == nil && resp.StatusCode == 200
		label := "联网核对"
		if resp != nil {
			defer resp.Body.Close()
			if resp.StatusCode == 422 {
				return nil, errors.New("分集身份或来源结构已变化，需要重新核对")
			}
		}
		if live {
			if err = json.NewDecoder(io.LimitReader(resp.Body, 65000)).Decode(&docs); err != nil {
				return nil, err
			}
		} else {
			label = "联网暂不可用，使用已核验快照"
			run := j.Base.Chapter.Research.Run
			if j.Base.ResearchRun != "" {
				run = j.Base.ResearchRun
			}
			if err := db.QueryRow(ctx, queries.ResearchDocuments, run).Scan(&docs); err != nil {
				return nil, err
			}
		}
		for _, d := range docs {
			if d.Episode != 1 || d.SHA256 != fmt.Sprintf("%x", sha256.Sum256([]byte(d.Text))) || !strings.HasPrefix(d.URL, "https://") {
				return nil, errors.New("分集来源指纹、集号或地址不匹配")
			}
			result = append(result, editSource{d.ID, d.URL, label + " · " + d.Locator + " · " + d.Fetched, d.Text, d.SHA256})
		}
		if len(result) < 2 {
			return nil, errors.New("缺少官方或独立分集资料")
		}
	}
	if len(result) == 0 {
		return nil, errors.New("缺少此章节的原创设定")
	}
	return result, nil
}
func checkChanges(j editorJob, changes []editChange) error {
	fields := j.Base.fields()
	seen := map[string]bool{}
	if len(changes) == 0 || len(changes) > len(fields) {
		return errors.New("请返回有实际修改的字段")
	}
	for _, d := range changes {
		old, ok := fields[d.Key]
		if !ok || seen[d.Key] || strings.TrimSpace(d.Text) == "" || old == d.Text || utf8.RuneCountInString(d.Text) > 1200 {
			return fmt.Errorf("字段或正文无效：%.80s", d.Key)
		}
		seen[d.Key] = true
		match := false
		for _, s := range j.Sources {
			if d.Source == s.ID && utf8.RuneCountInString(strings.TrimSpace(d.Quote)) >= 4 && strings.Contains(s.Text, d.Quote) {
				match = true
			}
		}
		if !match {
			return fmt.Errorf("字段 %s 的引用须逐字出现在指定来源中", d.Key)
		}
	}
	return nil
}
func editorModel(ctx context.Context, endpoint string, j editorJob) (json.RawMessage, error) {
	input := map[string]any{"instruction": j.Instruction, "feedback": j.Feedback, "issue": j.Issue, "work": j.World, "chapter": j.Chapter, "fields": j.Base.fields(), "sources": j.Sources, "draft": j.Draft}
	if j.Status == "checking" {
		// Pair the exact proposed text with its previous version. Unrelated live fields are outside the review.
		fields := j.Base.fields()
		type comparison struct {
			editChange
			Before string `json:"previous_published_text"`
		}
		rows := make([]comparison, 0, len(j.Draft))
		for _, d := range j.Draft {
			rows = append(rows, comparison{d, fields[d.Key]})
		}
		delete(input, "fields")
		input["draft"] = rows
	}
	if j.Base.ProfileFields != nil {
		input["scope"] = "人物介绍与世界观。person/<slug> 是本章登场人物的介绍，world/setting 是作品页世界观。只修改意见要求的字段；用本次取证范围内的事实，不提前透露后续剧情。"
	}
	body, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	callCtx, cancel := context.WithTimeout(ctx, 150*time.Second)
	defer cancel()
	u := fmt.Sprintf("%s/internal/editor/%s/%d/%d/%s", strings.TrimRight(endpoint, "/"), j.ID, j.Round, j.Attempt, j.Status)
	req, err := http.NewRequestWithContext(callCtx, "POST", u, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var raw json.RawMessage
	if err = json.NewDecoder(io.LimitReader(resp.Body, 64000)).Decode(&raw); err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		var problem struct{ Error string }
		_ = json.Unmarshal(raw, &problem)
		return nil, fmt.Errorf("Pi 编辑步骤 HTTP %d：%.240s", resp.StatusCode, problem.Error)
	}
	return raw, nil
}

// Each transition and its audit event commit together. A lost lease cannot replace a later draft.
func ProcessEditorial(ctx context.Context, db *pgxpool.Pool, endpoint string) error {
	var j editorJob
	err := db.QueryRow(ctx, queries.ClaimEditorTask).Scan(&j.ID, &j.World, &j.Chapter, &j.Instruction, &j.Feedback, &j.Issue, &j.Status, &j.Lease, &j.Round, &j.Attempt, &j.Retries, &j.Base, &j.Sources, &j.Draft)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	next, note, issue := "drafting", "", ""
	attempt, retries := j.Attempt, j.Retries
	var detail any
	if j.Status == "evidence" {
		j.Sources, err = editorialSources(ctx, db, j, endpoint)
		note = "已核对原创定稿与场景设定"
		if j.World != "tidepost" {
			note = "已核对官方与分集资料的原文、来源和指纹"
			if len(j.Sources) > 0 {
				note += " · " + j.Sources[0].Locator
			}
		}
		detail = j.Sources
	} else {
		var raw json.RawMessage
		raw, err = editorModel(ctx, endpoint, j)
		if err == nil && j.Status == "drafting" {
			var proposed []editChange
			err = json.Unmarshal(raw, &proposed)
			if err == nil {
				err = checkChanges(j, proposed)
			}
			if err == nil {
				j.Draft = proposed
				next = "checking"
				note = "修订已生成，字段和逐字引文检查通过"
				detail = proposed
			}
		} else if err == nil {
			var verdict struct {
				Pass *bool
				Note string
			}
			err = json.Unmarshal(raw, &verdict)
			if err == nil && (verdict.Pass == nil || strings.TrimSpace(verdict.Note) == "") {
				err = errors.New("校对模型未给出明确结论")
			}
			if err == nil {
				note = verdict.Note
				detail = j.Draft
				if *verdict.Pass {
					next = "review"
				} else {
					err = errors.New(note)
				}
			}
		}
	}
	if err != nil {
		issue = err.Error()
		note = issue
		retries++
		next = "drafting"
		if j.Status == "evidence" {
			next = "evidence"
		}
		if retries >= 3 {
			next = "failed"
		} else {
			attempt++
			note = "自动返工 · " + issue
		}
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		var locked string
		if err := tx.QueryRow(ctx, queries.CheckEditorLease, j.ID, j.Lease).Scan(&locked); err != nil {
			return err
		}
		if err := RecordEditorEvent(ctx, tx, j.ID, j.Status, note, detail); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, queries.AdvanceEditorTask, j.ID, next, attempt, retries, j.Sources, j.Draft, issue)
		return err
	})
}
