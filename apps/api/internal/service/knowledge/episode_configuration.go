package knowledge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"time"

	q "anime-agent/api/internal/repository/queries/knowledge"
	"github.com/jackc/pgx/v5"
)

type EpisodeConfiguration struct {
	Spec      json.RawMessage    `json:"spec"`
	Documents []researchDocument `json:"documents"`
}

func RegisterEpisodeConfiguration(ctx context.Context, tx pgx.Tx, request string, revision, number int, template json.RawMessage, result EpisodeConfiguration) error {
	var world, stage, title, edition string
	var paused bool
	var reason *string
	var actualRevision int
	var gaps []int32
	var reserved int64
	if _, err := tx.Exec(ctx, q.LockPlanRegistration); err != nil {
		return err
	}
	if err := tx.QueryRow(ctx, q.Configuration["PlanLock"], request).Scan(&world, &actualRevision, &paused, &reason, &stage, &gaps, &reserved); err != nil {
		return err
	}
	if paused || reason != nil || stage != "plan_validated" || revision != actualRevision || !slices.Contains(gaps, int32(number)) {
		return errors.New("制作计划或缺集范围已变化")
	}
	var exists bool
	if err := tx.QueryRow(ctx, q.Configuration["Registered"], world, number).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return errors.New("该集已有配置或定稿，保留原记录")
	}
	var spec, previous episodeSpec
	if json.Unmarshal(result.Spec, &spec) != nil || json.Unmarshal(template, &previous) != nil || spec.Number != number {
		return errors.New("分集配置与盘点集号不一致")
	}
	expectedGuide, _, _ := strings.Cut(previous.Guide, "#")
	if spec.Official != previous.Official || spec.Guide != fmt.Sprintf("%s#ep%d", expectedGuide, number) ||
		!reflect.DeepEqual(spec.SourceRules, previous.SourceRules) || spec.SourceRules == nil || spec.SourceNumber != 0 {
		return errors.New("来源超出已登记的同版本规则")
	}
	if err := spec.validate(); err != nil {
		return err
	}
	if err := validateSourceDocuments(result.Documents, number, number, &spec); err != nil {
		return err
	}
	var guide, official []string
	oldest := time.Now()
	for _, doc := range result.Documents {
		if doc.Fetched.Before(oldest.Add(-24*time.Hour)) || doc.Fetched.After(time.Now().Add(time.Minute)) {
			return errors.New("来源快照时间异常")
		}
		if doc.Fetched.Before(oldest) {
			oldest = doc.Fetched
		}
		if strings.HasPrefix(doc.ID, "guide-") {
			guide = doc.Sentences
		} else {
			official = doc.Sentences
		}
	}
	starts := make([]int, 0, len(spec.Beats))
	next := 0
	for _, beat := range spec.Beats {
		match := slices.IndexFunc(guide[next:], func(sentence string) bool { return strings.Contains(sentence, beat.Needle) })
		if match < 0 || beat.OfficialSentence < 0 || beat.OfficialSentence > len(official) {
			return errors.New("配置无法按顺序定位原文")
		}
		starts = append(starts, next+match)
		next += match + 1
	}
	var castProof struct {
		People []struct{ Evidence string }
	}
	if err := json.Unmarshal(result.Spec, &castProof); err != nil {
		return err
	}
	for i, person := range spec.People {
		index := person.Beat - 1
		end := len(guide)
		if index+1 < len(starts) {
			end = starts[index+1]
		}
		evidence := castProof.People[i].Evidence
		inGuide := slices.ContainsFunc(guide[starts[index]:end], func(sentence string) bool {
			return strings.Contains(sentence, evidence)
		})
		officialIndex := spec.Beats[index].OfficialSentence - 1
		inOfficial := officialIndex >= 0 && strings.Contains(official[officialIndex], evidence)
		if len([]rune(strings.TrimSpace(evidence))) < 3 || (!inGuide && !inOfficial) {
			return errors.New("人物出场缺少同段来源定位")
		}
	}
	type portraitRef struct{ ReferenceURL, SourceSHA string }
	var original portraitRef
	var art struct {
		portraitRef
		Character, Brief string
	}
	var refs struct{ PortraitReferences map[string]portraitRef }
	if json.Unmarshal(previous.Art, &original) != nil || json.Unmarshal(spec.Art, &art) != nil || json.Unmarshal(template, &refs) != nil {
		return errors.New("图片参考记录无效")
	}
	expected := original
	if len(refs.PortraitReferences) > 0 {
		var ok bool
		expected, ok = refs.PortraitReferences[art.Character]
		if !ok || !slices.ContainsFunc(spec.People, func(p planCast) bool {
			return p.Slug == art.Character && p.Beat == spec.SceneBeat && strings.Contains(art.Brief, p.Name)
		}) {
			return errors.New("场景人物、段落和肖像不一致")
		}
	}
	if expected != art.portraitRef || expected.ReferenceURL == "" {
		return errors.New("生成配置改变了已登记的图片参考")
	}
	allowed := map[string]string{}
	for _, p := range previous.People {
		allowed[p.Slug] = p.Name
	}
	cast := make([]planCast, 0, len(spec.People))
	for _, p := range spec.People {
		if allowed[p.Slug] == "" || allowed[p.Slug] != p.Name {
			return errors.New("角色没有可复用的已登记肖像")
		}
		cast = append(cast, planCast{Slug: p.Slug, Name: p.Name, ReusePortrait: true})
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(result.Spec, &fields); err != nil {
		return err
	}
	// Automatic registration produces a locally reviewable acceptance. Remote release is a separate opt-in.
	fields["publishToReader"] = json.RawMessage("false")
	raw, err := json.Marshal(fields)
	if err != nil {
		return err
	}
	if err = tx.QueryRow(ctx, q.Configuration["Work"], world).Scan(&title, &edition); err != nil {
		return err
	}
	if err = registerEdition(ctx, tx, editionPlan{World: world, Title: title, Edition: edition, People: cast, Episodes: []json.RawMessage{raw}}); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, q.Configuration["Attach"], request, revision, world, number, raw, result.Documents); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, q.SaveCapture, world, raw, result.Documents, oldest)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return errors.New("来源快照未登记")
	}
	if _, err = tx.Exec(ctx, q.Configuration["Enable"], fmt.Sprintf("%s-ep-%03d", world, number)); err != nil {
		return err
	}
	var more bool
	if err = tx.QueryRow(ctx, q.Configuration["More"], request).Scan(&more); err != nil {
		return err
	}
	if !more {
		_, err = tx.Exec(ctx, q.Configuration["Stage"], request, revision, "executing")
	}
	return err
}
