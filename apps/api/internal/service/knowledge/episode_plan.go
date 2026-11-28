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

	queries "anime-agent/api/internal/repository/queries/knowledge"
	"anime-agent/api/internal/repository/queries/ops"
)

type planCast struct {
	Slug, Name, Role string
	Beat             int
	ReusePortrait    bool
	Portrait         json.RawMessage
}
type sourceRules struct {
	Entry, Heading, Attribute, Body, Guide string
	Version                                int
	OfficialRule, GuideRule                *episodeSourceRule
}

type episodeSourceRule struct {
	Kind, Episode, Title, Heading, Body, TitleSelector string
}

var episodeJSONField = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

type episodeSpec struct {
	SourceRules                                                                                 *sourceRules
	Number, SceneBeat, SourceNumber                                                             int
	Slug, Title, Intro, OfficialTitle, GuideTitle, Official, Guide, SceneSlug, SceneName, Place string
	People                                                                                      []planCast
	Art                                                                                         json.RawMessage
	Beats                                                                                       []struct {
		Kind, Needle, Question string
		OfficialSentence       int
	}
	GuideParagraphs []string
	Setting         []json.RawMessage
}
type editionPlan struct {
	Automation                                               *episodePolicy
	World, Title, Edition, Provider, ExternalID, Intro, Type string
	Complete                                                 bool
	Year                                                     int
	People                                                   []planCast
	Episodes                                                 []json.RawMessage
}

var (
	planSlug           = regexp.MustCompile("^[a-z0-9]+(-[a-z0-9]+)*$")
	pressID            = regexp.MustCompile("^id=[0-9]{10}$")
	pressGuidePath     = regexp.MustCompile("^/[a-z0-9-]+/$")
	numericArchivePath = regexp.MustCompile("^/archives/[0-9]+/?$")
	tvTimesEpisodePath = regexp.MustCompile("^/dramas/[a-z0-9]+/[0-9]+/?$")
)

func (e episodeSpec) validate() error {
	if e.Number < 1 || !planSlug.MatchString(e.Slug) || !planSlug.MatchString(e.SceneSlug) || e.Title == "" || e.SceneName == "" || e.Place == "" || e.Intro == "" {
		return errors.New("缺少集号、稳定路由、标题或场景")
	}
	if e.SourceRules != nil {
		if err := e.validateRules(); err != nil {
			return err
		}
	} else if err := e.validateLegacySources(); err != nil {
		return err
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

// Legacy manifests are immutable, including the source checks used at registration.
func (e episodeSpec) validateLegacySources() error {
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
	apothecaryReleases := map[int]string{49: "1790842934", 50: "1791440404", 51: "1792047363"}
	release, found := apothecaryReleases[e.SourceNumber]
	apothecary := found && e.SourceNumber == e.Number+48 && e.Official == "https://www.animatetimes.com/news/details.php?id="+release
	council := e.Number <= 3 && e.Official == fmt.Sprintf("https://nama-anaru.com/episode/?id=%d", e.Number)
	pressRelease := official.Host == "www.animatetimes.com" && official.Path == "/news/details.php" && pressID.MatchString(official.RawQuery)
	itemPremiere := e.Number == 1 && e.Official == "https://toaru-project.com/item/story/01.html"
	romelia := e.Official == "https://romelia-senki.com/news/episode_data"
	aniplex := e.Number <= 2 && (e.Official == "https://www.aniplex.co.jp/lineup/majieku/story/" ||
		e.Official == "https://www.aniplex.co.jp/lineup/saikyosoubi/story/")
	yukionnaPremiere := e.Number == 1 && e.Official == "https://anime-yukionna.af-original.com/story/"
	lastBossPremiere := e.Number == 1 && e.SourceNumber == 13 && e.Official == "https://www.lastboss-anime.com/story/13.html"
	yukionnaSecond := e.Number == 2 && e.Official == "https://animestore.docomo.ne.jp/animestore/ci_pc?partId=29247002&workId=29247"
	lastBossSecond := e.Number == 2 && e.SourceNumber == 14 && e.Official == "https://www.lastboss-anime.com/story/14.html"
	darkSummonerPremiere := e.Number == 1 && e.Official == "https://darksummoner.deregula.com/story/"
	darkSummonerSecond := e.Number == 2 && e.Official == "https://animestore.docomo.ne.jp/animestore/ci_pc?partId=29245002&workId=29245"
	fxSecond := e.Number == 2 && e.Official == "https://fxkurumi-info.com/story/episodes.json"
	witchSecond := e.Number == 2 && e.Official == "https://www.b-ch.com/titles/9932/002"
	press := len(e.GuideParagraphs) > 0 && (pressRelease || itemPremiere || romelia || aniplex || yukionnaPremiere || lastBossPremiere || lastBossSecond || yukionnaSecond || darkSummonerPremiere || darkSummonerSecond || fxSecond || witchSecond) && official.Fragment == ""
	bebop := e.Official == "https://www.cowboy-bebop.net/story/" && e.Guide == fmt.Sprintf("https://en.wikipedia.org/wiki/List_of_Cowboy_Bebop_episodes#ep%d", e.Number) && e.SourceNumber == 0
	if !bebop && !naruto && !takopi && !lookBack && !apothecary && !council && !press {
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
		guideAnchor = fmt.Sprintf("ep%d", e.SourceNumber)
		if e.Guide != "https://en.wikipedia.org/wiki/List_of_The_Apothecary_Diaries_episodes#"+guideAnchor {
			return errors.New("《药屋》分集表来源不匹配")
		}
	}
	if press {
		if err != nil || guide.User != nil || guide.Scheme != "https" || guide.RawQuery != "" || guide.Fragment != "" ||
			!((guide.Host == "2jigen-otaku.com" && pressGuidePath.MatchString(guide.Path)) ||
				(guide.Host == "tv-times.jp" && tvTimesEpisodePath.MatchString(guide.Path)) ||
				((guide.Host == "saisokustory.com" || guide.Host == "animenoyakata.com") && pressGuidePath.MatchString(guide.Path)) ||
				(yukionnaPremiere && e.Guide == "https://ani-chara.net/18988/") ||
				(e.Official == "https://www.animatetimes.com/news/details.php?id=1789112810" && e.Guide == "https://sayo.blog/tenken13/") ||
				(lastBossPremiere && e.Guide == "https://ani-chara.net/18776/") ||
				(lastBossSecond && e.Guide == "https://ani-chara.net/19514/") ||
				(yukionnaSecond && e.Guide == "https://ani-chara.net/19869/") ||
				(e.Number == 2 && e.Official == "https://www.aniplex.co.jp/lineup/majieku/story/" && e.Guide == "https://ani-chara.net/19543/") ||
				(darkSummonerPremiere && e.Guide == "https://ani-chara.net/18983/") ||
				(darkSummonerSecond && e.Guide == "https://ani-chara.net/19691/") ||
				(romelia && e.Number == 2 && e.Guide == "https://biwaochan-blog.com/romelia-senki-2-kansou/") ||
				(e.Number == 2 && e.Official == "https://www.animatetimes.com/news/details.php?id=1791607498" && e.Guide == "https://sayo.blog/tenken14/") ||
				(e.Number == 2 && e.Official == "https://www.animatetimes.com/news/details.php?id=1791814021" && e.Guide == "https://ani-chara.net/20072/") ||
				(e.Number == 2 && e.SourceNumber == 26 && e.Official == "https://www.animatetimes.com/news/details.php?id=1791447769" && e.Guide == "https://ani-chara.net/19554/") ||
				((guide.Host == "anime-no-mikata.com" || guide.Host == "mahoyaku.net") && numericArchivePath.MatchString(guide.Path))) {
			return errors.New("新闻稿分集补充来源不匹配")
		}
		if len(e.GuideParagraphs) < 3 || len(e.GuideParagraphs) > 18 {
			return errors.New("补充分集需要 3–18 个原文段落定位")
		}
		for _, start := range e.GuideParagraphs {
			if strings.TrimSpace(start) != start || len(start) < 8 || len(start) > 320 {
				return errors.New("分集资料段落定位无效")
			}
		}
	} else if council {
		if e.Guide != fmt.Sprintf("https://2jigen-otaku.com/seitokai-ana-%dwa/", e.Number) || (e.Number == 3 && (len(e.GuideParagraphs) < 3 || len(e.GuideParagraphs) > 18)) {
			return errors.New("TV 版分集补充来源不匹配")
		}
	} else if err != nil || guide.Scheme != "https" || guide.Host != "en.wikipedia.org" || !strings.HasPrefix(guide.Path, "/wiki/") || guide.Fragment != guideAnchor {
		return errors.New("分集指南与集号不一致")
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
	err := tx.QueryRow(ctx, queries.RegisterArt, w.World, kind, slug, spec).Scan(&same)
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
		err := tx.QueryRow(ctx, queries.ResolveEditionEpisodeCount, w.Provider, w.ExternalID, kind, w.Year).Scan(&count)
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
	err := tx.QueryRow(ctx, queries.FindWorldVersion, w.World).Scan(&v)
	if errors.Is(err, pgx.ErrNoRows) {
		var work string
		err = tx.QueryRow(ctx, queries.ResolveEditionWork, w.Provider, w.ExternalID, w.Year, kind).Scan(&work)
		if err != nil {
			return fmt.Errorf("对照 canonical 作品、类型与年份: %w", err)
		}
		meta, _ := json.Marshal([]string{w.World, w.Title, w.Edition, work, w.Intro})
		sha := digest(meta)
		err = tx.QueryRow(ctx, queries.CreateEditionVersion, w.World, sha, meta).Scan(&v)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, queries.CreateWorldRelease, v, work); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, queries.RegisterEditionWorld, work, v, w.World, w.Title, w.Edition, w.Intro); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, ops.SeedOpsMediaBatchWorlds); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, queries.AppendHomeSlot, work); err != nil {
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
		if _, err = tx.Exec(ctx, queries.RegisterTopic, topic, w.World, e.Number); err != nil {
			return err
		}
		var same bool
		err = tx.QueryRow(ctx, queries.RegisterPlan, topic, w.World, e.Number, raw).Scan(&same)
		if err != nil {
			return err
		}
		if !same {
			return fmt.Errorf("第 %d 集已有锁定的选题和链接", e.Number)
		}
		if _, err = tx.Exec(ctx, queries.RegisterExtension, topic, raw); err != nil {
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
			if p.ReusePortrait {
				var found bool
				if err = tx.QueryRow(ctx, queries.HasPortrait, w.World, p.Slug).Scan(&found); err != nil {
					return err
				}
				if !found {
					return fmt.Errorf("缺少待复用的肖像 %s/%s", w.World, p.Slug)
				}
				continue
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
			if _, e := tx.Exec(ctx, queries.LockPlanRegistration); e != nil {
				return e
			}
			sum := digest(raw)
			var loaded bool
			if err := tx.QueryRow(ctx, queries.HasManifest, w.World, sum).Scan(&loaded); err != nil || loaded {
				return err
			}
			if err := registerEdition(ctx, tx, w); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, queries.SaveManifest, w.World, sum)
			return err
		})
		if e != nil {
			return fmt.Errorf("%s: %w", filepath.Base(p), e)
		}
	}
	return nil
}

// Each topic owns its immutable reader text and asset selections. Later topics never
// advance an earlier head. Withdrawals are applied independently by the public views.
func AcceptPlannedEpisodes(ctx context.Context, db *pgxpool.Pool) error {
	rows, err := db.Query(ctx, queries.ListDueAcceptances)
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
			if e := tx.QueryRow(ctx, queries.TryLockEpisode, topic).Scan(&locked); e != nil || !locked {
				return e
			}
			var spec, raw []byte
			var batch, stage, reason string
			var already bool
			err := tx.QueryRow(ctx, queries.AcceptanceState, topic).Scan(&spec, &batch, &stage, &reason, &already)
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
			if err = tx.QueryRow(ctx, queries.AcceptanceInputs, topic, batch, ids, e.Number, e.Number).Scan(&raw); err != nil {
				return err
			}
			var in scopeInput
			if err = json.Unmarshal(raw, &in); err != nil {
				return err
			}
			policy, perr := e.evidence()
			valid := perr == nil && in.Research != "" && validateSourceDocuments(in.Documents, e.Number, e.Number, &e) == nil && validateEpisodeClaims(in.Claims, in.Documents, e.Number, e.Number, policy) == nil && len(in.Claims) == len(e.Beats)
			checks := []scopeCheck{{Name: "人物"}, {Name: "剧情"}, {Name: "世界观"}, {Name: "时间线"}}
			gap := "等待同集、双来源的取证校对 · " + stage
			if policy.Documents == 1 {
				gap = "等待同集、官方来源的取证校对 · " + stage
			}
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
					tag, err := tx.Exec(ctx, queries.BumpResearchPolicy, in.Research)
					if err != nil {
						return err
					}
					if tag.RowsAffected() == 1 {
						return researchEvent(ctx, tx, in.Research, "验收退回，自动重新取证校对："+strings.Join(gaps, "；"))
					}
				}
			}
			if len(picks) > 0 && e.Number == 1 {
				_, err = tx.Exec(ctx, queries.SetAcceptedWorldCover, in.World, picks[0].Image, topic)
			}
			return err
		})
		if err != nil {
			return fmt.Errorf("%s: %w", topic, err)
		}
	}
	return nil
}

// These are publisher origins, not a per-episode URL whitelist. The database
// stores the edition, source pair, DOM rules and independently printed titles.
func (e episodeSpec) validateRules() error {
	r := e.SourceRules
	official, err := url.Parse(e.Official)
	if err != nil || official.Scheme != "https" || official.User != nil || official.Fragment != "" || official.Path == "" {
		return errors.New("官方来源无效")
	}
	switch official.Host {
	case "www.cowboy-bebop.net", "mantan-web.jp", "www.animatetimes.com", "www.tbs.co.jp", "naruto-official.com", "nama-anaru.com", "www.aniplex.co.jp", "lookback-anime.com", "www.lastboss-anime.com", "fxkurumi-info.com", "romelia-senki.com", "www.b-ch.com", "animestore.docomo.ne.jp", "toaru-project.com", "anime-yukionna.af-original.com", "darksummoner.deregula.com", "horemajo-anime.com":
	default:
		return errors.New("未登记的来源站点")
	}
	if r.Guide == "official_only" {
		if e.Guide != "" || len(e.GuideParagraphs) != 0 || r.Version != 1 || r.OfficialRule == nil || !r.OfficialRule.valid() ||
			r.GuideRule != nil || r.Entry != "" || r.Heading != "" || r.Attribute != "" || r.Body != "" ||
			strings.TrimSpace(e.OfficialTitle) == "" {
			return errors.New("单官方来源规则无效")
		}
		return nil
	}
	if r.Guide == "paired" {
		guide, err := url.Parse(e.Guide)
		if err != nil || guide.Scheme != "https" || guide.User != nil || guide.Host == official.Host || guide.Fragment != "" || guide.RawQuery != "" || guide.Path == "" {
			return errors.New("独立分集来源无效")
		}
		switch guide.Host {
		case "anibanashi.com", "skypenguin.net", "2jigen-otaku.com", "sayo.blog", "ani-chara.net", "tv-times.jp", "saisokustory.com", "animenoyakata.com", "anime-no-mikata.com", "mahoyaku.net", "biwaochan-blog.com":
		default:
			return errors.New("未登记的分集资料站点")
		}
		if r.Version != 1 || !r.OfficialRule.valid() || !r.GuideRule.valid() || len(e.GuideParagraphs) != 0 ||
			strings.TrimSpace(e.OfficialTitle) == "" || r.Entry != "" || r.Heading != "" || r.Attribute != "" || r.Body != "" {
			return errors.New("分集采集规则无效")
		}
		return nil
	}
	for _, s := range []string{r.Entry, r.Heading, r.Body, e.OfficialTitle} {
		if len(s) == 0 || len(s) > 512 {
			return errors.New("缺少来源定位或标题")
		}
	}
	if r.Attribute != "" && r.Attribute != "alt" && r.Attribute != "title" && r.Attribute != "content" {
		return errors.New("来源标题属性无效")
	}
	guide, err := url.Parse(e.Guide)
	if err != nil || guide.Scheme != "https" || guide.User != nil || guide.RawQuery != "" {
		return errors.New("分集资料无效")
	}
	n := e.Number
	if e.SourceNumber != 0 {
		n = e.SourceNumber
	}
	if n < 1 {
		return errors.New("无效来源集号")
	}
	switch r.Guide {
	case "wiki-episode":
		if guide.Host != "en.wikipedia.org" || !strings.HasPrefix(guide.Path, "/wiki/") || guide.Fragment != fmt.Sprintf("ep%d", n) || len(e.GuideParagraphs) > 0 {
			return errors.New("分集资料锚点与来源集号不符")
		}
	case "paragraphs":
		switch guide.Host {
		case "2jigen-otaku.com", "sayo.blog", "ani-chara.net", "tv-times.jp", "saisokustory.com", "animenoyakata.com", "anime-no-mikata.com", "mahoyaku.net", "biwaochan-blog.com":
		default:
			return errors.New("未登记的分集资料站点")
		}
		if guide.Fragment != "" || len(e.GuideParagraphs) < 3 || len(e.GuideParagraphs) > 18 {
			return errors.New("分集资料需要 3–18 个段落定位")
		}
		for _, start := range e.GuideParagraphs {
			if len(strings.TrimSpace(start)) < 8 || len(start) > 320 {
				return errors.New("无效段落定位")
			}
		}
	default:
		return errors.New("未知分集资料解析规则")
	}
	return nil
}

func (r *episodeSourceRule) valid() bool {
	if r == nil || strings.TrimSpace(r.Body) == "" || len(r.Body) > 256 {
		return false
	}
	switch r.Kind {
	case "json_episode_list":
		return r.Heading == "" && r.TitleSelector == "" && len(r.Episode) <= 256 && len(r.Title) <= 256 &&
			episodeJSONField.MatchString(r.Episode) && episodeJSONField.MatchString(r.Title) && episodeJSONField.MatchString(r.Body)
	case "html_episode_article":
		return r.Episode == "" && r.Title == "" && strings.TrimSpace(r.Heading) != "" && len(r.Heading) <= 256 &&
			len(r.TitleSelector) <= 256
	default:
		return false
	}
}
