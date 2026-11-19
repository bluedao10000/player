package knowledge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type planCast struct {
	Slug, Name, Role string
	Beat             int
	Portrait         json.RawMessage
}
type episodeSpec struct {
	Number, SceneBeat, SourceNumber                                                             int
	Slug, Title, Intro, OfficialTitle, GuideTitle, Official, Guide, SceneSlug, SceneName, Place string
	People                                                                                      []planCast
	Art                                                                                         json.RawMessage
	Beats                                                                                       []struct {
		Kind, Needle, Question string
		OfficialSentence       int
	}
	Setting []json.RawMessage
}
type editionPlan struct {
	World, Title, Edition, Provider, ExternalID, Intro, Type string
	Complete                                                 bool
	Year                                                     int
	People                                                   []planCast
	Episodes                                                 []json.RawMessage
}

var planSlug = regexp.MustCompile("^[a-z0-9]+(-[a-z0-9]+)*$")

func (e episodeSpec) validate() error {
	if e.Number < 1 || !planSlug.MatchString(e.Slug) || !planSlug.MatchString(e.SceneSlug) || e.Title == "" || e.SceneName == "" || e.Place == "" || e.Intro == "" {
		return errors.New("缺少集号、稳定路由、标题或场景")
	}
	official, err := url.Parse(e.Official)
	tbsPath := fmt.Sprintf("/anime/takopi_project/series/story/story%02d.html", e.Number)
	if e.Number == 6 {
		tbsPath = "/anime/takopi_project/series/story/index.html"
	}
	if err != nil || official.User != nil || official.Scheme != "https" || e.OfficialTitle == "" {
		return errors.New("官方来源或标题无效")
	}
	naruto := official.Host == "naruto-official.com" && strings.HasPrefix(official.Path, "/en/anime/")
	takopi := official.Host == "www.tbs.co.jp" && official.Path == tbsPath && e.Number <= 6 && official.RawQuery == "" && official.Fragment == ""
	lookBack := e.Number == 1 && e.Official == "https://lookback-anime.com/"
	apothecary := e.Number == 1 && e.SourceNumber == 49 && e.Official == "https://www.animatetimes.com/news/details.php?id=1790842934"
	council := e.Number == 1 && e.Official == "https://nama-anaru.com/episode/?id=1"
	if !naruto && !takopi && !lookBack && !apothecary && !council {
		return errors.New("官方来源或标题无效")
	}
	guide, err := url.Parse(e.Guide)
	guideAnchor := fmt.Sprintf("ep%d", e.Number)
	if lookBack {
		guideAnchor = "Plot"
		if e.Guide != "https://en.wikipedia.org/wiki/Look_Back_(2024_film)#Plot" {
			return errors.New("电影版剧情来源不匹配")
		}
	}
	if apothecary {
		guideAnchor = "ep49"
		if e.Guide != "https://en.wikipedia.org/wiki/List_of_The_Apothecary_Diaries_episodes#ep49" {
			return errors.New("第49话分集表来源不匹配")
		}
	}
	if council {
		if e.Guide != "https://2jigen-otaku.com/seitokai-ana-1wa/" {
			return errors.New("TV第1话补充来源不匹配")
		}
	} else if err != nil || guide.Scheme != "https" || guide.Host != "en.wikipedia.org" || !strings.HasPrefix(guide.Path, "/wiki/") || guide.Fragment != guideAnchor {
		return errors.New("分集指南与集号不一致")
	}
	if e.GuideTitle == "" {
		return errors.New("分集指南缺少标题")
	}
	if len(e.Beats) < 4 || len(e.Beats) > 6 || e.SceneBeat < 1 || e.SceneBeat > len(e.Beats) || len(e.People) == 0 {
		return errors.New("需要人物、4–6 个剧情节点和有效场景段号")
	}
	kinds, needles := map[string]bool{}, map[string]bool{}
	officialQuote := false
	for _, b := range e.Beats {
		if b.Needle == "" || needles[b.Needle] || b.Question == "" || (b.Kind != "motive" && b.Kind != "turn" && b.Kind != "cause") {
			return errors.New("剧情节点缺少唯一的原文片段或取证问题")
		}
		kinds[b.Kind] = true
		needles[b.Needle] = true
		officialQuote = officialQuote || b.OfficialSentence > 0
	}
	if len(kinds) != 3 || !officialQuote {
		return errors.New("剧情节点应覆盖动机、转折、因果并定位至少一处官方原文")
	}
	var term string
	var beat int
	if len(e.Setting) != 2 || json.Unmarshal(e.Setting[0], &term) != nil || term == "" || json.Unmarshal(e.Setting[1], &beat) != nil || beat < 1 || beat > len(e.Beats) {
		return errors.New("世界设定缺少有效段号")
	}
	return nil
}

func registerArt(ctx context.Context, tx pgx.Tx, w editionPlan, kind, slug, title string, number int, raw json.RawMessage) error {
	var spec map[string]any
	if err := json.Unmarshal(raw, &spec); err != nil {
		return err
	}
	spec["kind"] = kind
	spec["title"] = title
	spec["episode"] = number
	spec["series"] = w.Title
	spec["world"] = w.World
	spec["slug"] = slug
	if !shaRE.MatchString(fmt.Sprint(spec["sourceSHA"])) || spec["brief"] == nil {
		return fmt.Errorf("缺少美术设定 %s", slug)
	}
	var same bool
	err := tx.QueryRow(ctx, `INSERT INTO episode_art_jobs(world_slug,kind,slug,spec) VALUES($1,$2,$3,$4) ON CONFLICT(world_slug,kind,slug) DO UPDATE SET slug=excluded.slug RETURNING spec=$4::jsonb`, w.World, kind, slug, spec).Scan(&same)
	if err == nil && !same {
		return fmt.Errorf("已有图片任务设定变化 %s", slug)
	}
	return err
}
func registerEdition(ctx context.Context, tx pgx.Tx, w editionPlan) error {
	if !planSlug.MatchString(w.World) || len(w.Episodes) == 0 || w.Edition == "" {
		return fmt.Errorf("无效作品选题")
	}
	kind := w.Type
	if kind == "" {
		kind = "TV"
	}
	if kind != "TV" && kind != "ONA" && kind != "MOVIE" {
		return errors.New("不支持的作品类型")
	}
	if w.Complete {
		var count int
		err := tx.QueryRow(ctx, `SELECT k.episodes FROM work_sources s JOIN works k ON k.id=s.work_id WHERE s.provider=$1 AND s.external_id=$2 AND k.active AND k.type=$3 AND k.season_year=$4 AND k.status='FINISHED'`, w.Provider, w.ExternalID, kind, w.Year).Scan(&count)
		if err != nil {
			return fmt.Errorf("核对已完结 canonical 作品的总集数: %w", err)
		}
		if count != len(w.Episodes) {
			return fmt.Errorf("全作品集数与目录不一致：目录 %d，选题 %d", count, len(w.Episodes))
		}
		for i, raw := range w.Episodes {
			var e episodeSpec
			if json.Unmarshal(raw, &e) != nil || e.Number != i+1 {
				return errors.New("整部选题必须从第 1 集连续排列")
			}
		}
	}
	var v int64
	err := tx.QueryRow(ctx, "SELECT version_id FROM work_worlds WHERE slug=$1", w.World).Scan(&v)
	if errors.Is(err, pgx.ErrNoRows) {
		var work string
		err = tx.QueryRow(ctx, `SELECT (SELECT DISTINCT k.id FROM work_sources s JOIN works k ON k.id=s.work_id WHERE s.provider=$1 AND s.external_id=$2 AND k.active AND k.type=$4 AND k.season_year=$3)::text`, w.Provider, w.ExternalID, w.Year, kind).Scan(&work)
		if err != nil {
			return fmt.Errorf("对照 canonical 作品、类型与年份: %w", err)
		}
		meta, _ := json.Marshal([]string{w.World, w.Title, w.Edition, work, w.Intro})
		sha := digest(meta)
		err = tx.QueryRow(ctx, `WITH r AS (INSERT INTO knowledge_sync_runs(source_id,status,finished_at,remote_version,checksum,records_read,records_created) SELECT id,'success',now(),$1,$2,1,1 FROM knowledge_sources WHERE name='episode-plans' RETURNING id,source_id)
  INSERT INTO knowledge_source_versions(source_id,version,checksum,metadata,run_id,parser_type,asset_name) SELECT source_id,$1,$2,$3,id,'episode-manifest',$1||'.json' FROM r RETURNING id`, w.World, sha, meta).Scan(&v)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, "INSERT INTO world_releases VALUES($1,$2)", v, work); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO work_worlds(work_id,version_id,slug,name,edition,intro,cast_title,origin) VALUES($1,$2,$3,$4,$5,$6,'登场人物','external')`, work, v, w.World, w.Title, w.Edition, w.Intro); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, "INSERT INTO home_world_slots SELECT coalesce(max(position),0)+1,$1 FROM home_world_slots", work); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	var cast = map[string]planCast{}
	for _, c := range w.People {
		if !planSlug.MatchString(c.Slug) || c.Name == "" || cast[c.Slug].Name != "" {
			return fmt.Errorf("人物标识无效")
		}
		cast[c.Slug] = c
	}
	for _, raw := range w.Episodes {
		var e episodeSpec
		if err = json.Unmarshal(raw, &e); err != nil {
			return err
		}
		if err = e.validate(); err != nil {
			return fmt.Errorf("第 %d 集配置无效: %w", e.Number, err)
		}
		topic := fmt.Sprintf("%s-ep-%03d", w.World, e.Number)
		if _, err = tx.Exec(ctx, "INSERT INTO research_topics(id,world_slug,first_episode,last_episode) VALUES($1,$2,$3,$3) ON CONFLICT DO NOTHING", topic, w.World, e.Number); err != nil {
			return err
		}
		var same bool
		err = tx.QueryRow(ctx, `INSERT INTO episode_plans(topic,world_slug,number,spec) VALUES($1,$2,$3,$4) ON CONFLICT(topic) DO UPDATE SET topic=excluded.topic RETURNING spec=$4::jsonb`, topic, w.World, e.Number, raw).Scan(&same)
		if err != nil {
			return err
		}
		if !same {
			return fmt.Errorf("第 %d 集已有锁定的选题和链接", e.Number)
		}
		if _, err = tx.Exec(ctx, `INSERT INTO episode_extensions(topic,slug,title,intro,place,scene_slug,scene_name,people) SELECT $1,s->>'slug',s->>'title',s->>'intro',s->>'place',s->>'sceneSlug',s->>'sceneName',s->'people' FROM (SELECT $2::jsonb s) x ON CONFLICT(topic) DO NOTHING`, topic, raw); err != nil {
			return err
		}
		if err = registerArt(ctx, tx, w, "scene", e.SceneSlug, e.SceneName, e.Number, e.Art); err != nil {
			return err
		}
		for _, c := range e.People {
			p, ok := cast[c.Slug]
			if !ok || c.Name != p.Name || c.Beat < 1 || c.Beat > len(e.Beats) {
				return fmt.Errorf("第 %d 集的人物定位无效", e.Number)
			}
			if err = registerArt(ctx, tx, w, "portrait", p.Slug, p.Name, 0, p.Portrait); err != nil {
				return err
			}
		}
	}
	return nil
}
func LoadEpisodePlans(ctx context.Context, db *pgxpool.Pool) error {
	dir := os.Getenv("EPISODE_PLAN_DIR")
	if dir == "" {
		return nil
	}
	entries, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return err
	}
	for _, p := range entries {
		raw, e := os.ReadFile(p)
		if e != nil {
			return e
		}
		var w editionPlan
		if e = json.Unmarshal(raw, &w); e != nil {
			return e
		}
		e = pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
			if _, e := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(82045125)"); e != nil {
				return e
			}
			sum := digest(raw)
			var loaded bool
			if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT FROM episode_plan_manifests WHERE world_slug=$1 AND checksum=$2)", w.World, sum).Scan(&loaded); err != nil || loaded {
				return err
			}
			if err := registerEdition(ctx, tx, w); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, "INSERT INTO episode_plan_manifests VALUES($1,$2,now()) ON CONFLICT(world_slug) DO UPDATE SET checksum=excluded.checksum,loaded_at=now()", w.World, sum)
			return err
		})
		if e != nil {
			return fmt.Errorf("%s: %w", filepath.Base(p), e)
		}
	}
	return nil
}

const enqueueEpisodeArt = `INSERT INTO media_candidates(slot,batch,position,scene,variant,stage,art_job)
 SELECT nextval('episode_candidate_slots'),'cfg-'||a.id,i,a.spec->>'title',i,'reference',a.id
 FROM episode_plans p JOIN episode_art_jobs a ON a.world_slug=p.world_slug CROSS JOIN LATERAL generate_series(1,CASE a.kind WHEN 'scene' THEN 3 ELSE 1 END)i
 WHERE p.topic=$1 AND ((a.kind='scene' AND a.slug=p.spec->>'sceneSlug') OR (a.kind='portrait' AND p.spec->'people' @> jsonb_build_array(jsonb_build_object('slug',a.slug))))
 AND NOT EXISTS(SELECT FROM media_candidates c WHERE c.art_job=a.id AND position=i) ON CONFLICT(batch,position) DO NOTHING`

// Each topic owns its immutable reader text and asset selections. Later topics never
// advance an earlier head. Withdrawals are applied independently by the public views.
func AcceptPlannedEpisodes(ctx context.Context, db *pgxpool.Pool) error {
	rows, err := db.Query(ctx, `SELECT p.topic FROM episode_plans p JOIN research_topics t ON t.id=p.topic AND t.enabled LEFT JOIN scope_heads h ON h.topic=p.topic WHERE h.published IS NULL ORDER BY p.number`)
	if err != nil {
		return err
	}
	topics, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return err
	}
	for _, topic := range topics {
		err = pgx.BeginTxFunc(ctx, db, pgx.TxOptions{IsoLevel: pgx.RepeatableRead}, func(tx pgx.Tx) error {
			var locked bool
			if e := tx.QueryRow(ctx, "SELECT pg_try_advisory_xact_lock(hashtextextended($1,72))", topic).Scan(&locked); e != nil || !locked {
				return e
			}
			var spec, raw []byte
			var batch, stage, reason string
			var already bool
			err := tx.QueryRow(ctx, `SELECT p.spec,'cfg-'||a.id,coalesce(r.stage,'queued'),coalesce(r.error,''),EXISTS(SELECT FROM scope_heads h WHERE h.topic=p.topic AND h.published IS NOT NULL)
 FROM episode_plans p JOIN episode_art_jobs a ON a.world_slug=p.world_slug AND a.kind='scene' AND a.slug=p.spec->>'sceneSlug'
 LEFT JOIN LATERAL(SELECT stage,error FROM research_runs WHERE topic=p.topic ORDER BY policy DESC,attempt DESC LIMIT 1) r ON true WHERE p.topic=$1`, topic).Scan(&spec, &batch, &stage, &reason, &already)
			if err != nil || already {
				return err
			}
			var e episodeSpec
			if err = json.Unmarshal(spec, &e); err != nil {
				return err
			}
			ids := []string{}
			for _, p := range e.People {
				ids = append(ids, p.Slug)
			}
			if err = tx.QueryRow(ctx, acceptanceInputs, topic, batch, ids, e.Number, e.Number).Scan(&raw); err != nil {
				return err
			}
			var in scopeInput
			if err = json.Unmarshal(raw, &in); err != nil {
				return err
			}
			valid := in.Research != "" && validateSourceDocuments(in.Documents, e.Number, e.Number, &e) == nil && validateEpisodeClaims(in.Claims, in.Documents, e.Number, e.Number) == nil && len(in.Claims) == len(e.Beats)
			checks := []scopeCheck{{Name: "人物"}, {Name: "剧情"}, {Name: "世界观"}, {Name: "时间线"}}
			gap := "等待同集、双来源的取证校对 · " + stage
			if reason != "" {
				gap += " · " + reason
			}
			checks[1].add(valid, "动机、转折与结局", fmt.Sprintf("%d 段导读已逐段匹配原文", len(in.Claims)), e.Guide, gap)
			ch := scopeChapter{Episode: e.Number, Slug: e.Slug, Scene: e.SceneSlug, Block: fmt.Sprintf("episode-%d", e.SceneBeat)}
			extract := func(i int) (string, string) {
				if valid {
					return in.Claims[i-1].Text, in.Claims[i-1].Cites[0].URL
				}
				return "", ""
			}
			for _, p := range e.People {
				text, link := extract(p.Beat)
				checks[0].add(strings.Contains(text, p.Name), p.Name, text, link, fmt.Sprintf("第 %d 段缺少可追溯的%s出场或行动", p.Beat, p.Name))
			}
			var setting string
			var beat int
			if err = json.Unmarshal(e.Setting[0], &setting); err != nil {
				return err
			}
			if err = json.Unmarshal(e.Setting[1], &beat); err != nil {
				return err
			}
			text, link := extract(beat)
			checks[2].add(strings.Contains(text, setting), setting, text, link, fmt.Sprintf("第 %d 段需要说明并佐证「%s」", beat, setting))
			previous := 0
			for i, b := range e.Beats {
				n, source := 0, ""
				if valid {
					n, source = eventSource(in.Documents, in.Claims[i], b.Needle)
					text, _ := extract(i + 1)
					key := ""
					if i+1 == e.SceneBeat {
						key = ch.Block
					}
					ch.Passages = append(ch.Passages, struct{ Text, Block string }{text, key})
				}
				checks[3].add(n > previous, b.Question, fmt.Sprintf("分集资料第 %d 句", n), source, "未匹配到按顺序出现的分集原文")
				previous = n
			}
			in.Chapters = []scopeChapter{ch}
			art := scopeCheck{Name: "图片"}
			for _, p := range e.People {
				a := in.Art[p.Slug]
				art.add(currentArt(a), p.Name+"肖像", "已匹配验图凭证与公开肖像指纹", "#"+batch, "等待人物参考、生成和肖像验图")
			}
			done := len(in.Images) == 3
			var state []string
			for _, c := range in.Images {
				done = done && (c.Stage == "passed" || c.Stage == "held" || c.Stage == "failed")
				state = append(state, fmt.Sprintf("%d · %s", c.Variant, c.Stage))
			}
			art.add(done, "三张原图与手机裁切完成比选", strings.Join(state, "，"), "#"+batch, "候选进度："+strings.Join(state, "，"))
			art, picks, err := selectScenes(in, batch, []sceneRule{{e.SceneName, e.SceneSlug, ch.Block, e.Slug}}, "#"+batch, art)
			if err != nil {
				return err
			}
			checks = append(checks, art)
			// Include the config itself in the audit fingerprint.
			if err = recordScope(ctx, tx, in, append(raw, spec...), topic, e.Number, e.Number, 1, checks, picks); err != nil {
				return err
			}
			if valid {
				var gaps []string
				for _, c := range checks[:4] {
					for _, item := range c.Items {
						if item.Gap != "" {
							gaps = append(gaps, item.Gap)
						}
					}
				}
				if len(gaps) > 0 {
					tag, err := tx.Exec(ctx, `UPDATE research_topics t SET policy=t.policy+1 FROM research_runs r WHERE r.id=$1 AND (r.topic,r.policy)=(t.id,t.policy) AND t.policy<3`, in.Research)
					if err != nil {
						return err
					}
					if tag.RowsAffected() == 1 {
						return researchEvent(ctx, tx, in.Research, "验收退回，自动重新取证校对："+strings.Join(gaps, "；"))
					}
				}
			}
			if len(picks) > 0 && e.Number == 1 {
				_, err = tx.Exec(ctx, `UPDATE work_worlds SET image=$2 WHERE slug=$1 AND image IS NULL AND EXISTS(SELECT FROM visible_reader_scopes WHERE topic=$3)`, in.World, picks[0].Image, topic)
			}
			return err
		})
		if err != nil {
			return fmt.Errorf("%s: %w", topic, err)
		}
	}
	return nil
}
