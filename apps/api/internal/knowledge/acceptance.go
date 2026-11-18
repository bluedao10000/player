package knowledge

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed acceptance_inputs.sql
var acceptanceInputs string

const scopeTopic = "bebop-tv-1998-01-02"
const scopeRule = 2

type scopePerson struct {
	Name, Bio string
	Episodes  []int
}
type scopeArt struct {
	Image, SHA, Credit string
	Verified           bool
}
type scopeChapter struct {
	Episode                               int
	Slug, Copy, Scene, Place, Block, Text string
	Sourced, Linked                       bool
	Passages                              []struct{ Text, Block string }
}
type scopeImage struct {
	Position, Slot, Variant, Round    int
	Name, Ref, Stage, Withdrawn, Crop string
	Generated                         struct{ Image, Phone, SHA, Reference_SHA, Model string }
	Review                            struct {
		Decision, Phone, Model, SecondModel string
		Checks                              map[string]bool
	}
}
type scopeInput struct {
	World, Setting, Research string
	Version                  int64
	Documents                []researchDocument
	Claims                   []researchClaim
	People                   map[string]scopePerson
	Art                      map[string]scopeArt
	Chapters                 []scopeChapter
	Images                   []scopeImage
}
type scopeItem struct {
	Label    string `json:"label"`
	Evidence string `json:"evidence,omitempty"`
	Source   string `json:"source,omitempty"`
	Gap      string `json:"gap,omitempty"`
}
type scopeCheck struct {
	Name  string      `json:"name"`
	Items []scopeItem `json:"items"`
}

func (c *scopeCheck) add(ok bool, label, proof, source, gap string) {
	item := scopeItem{Label: label}
	if ok {
		item.Evidence = proof
		item.Source = source
	} else {
		item.Gap = gap
	}
	c.Items = append(c.Items, item)
}

type scenePick struct {
	Name     string `json:"name"`
	Chapter  string `json:"chapter"`
	Block    string `json:"block"`
	Anchor   string `json:"anchor"`
	Slot     int    `json:"slot"`
	Round    int    `json:"round"`
	Variant  int    `json:"variant"`
	Image    string `json:"image"`
	SHA      string `json:"sha"`
	Phone    string `json:"phone"`
	PhoneSHA string `json:"phone_sha"`
}

// The event locators are within each guide's story synopsis, independent of air dates.
var episodeRules = []struct {
	slug, copy, scene, block, setting, needle string
	settingBeat                               int
	events, locators                          []string
}{
	{"asteroid-blues", "9e179ac73218d6d0aa81efc4a9a9eafdb861e5496979abefd0844f1cd01b0ac4", "bar-visit", "bar-deal", "血腥之眼如何改变酒吧交易", "using the drug himself", 2,
		[]string{"接下提华纳悬赏", "酒吧交易被打断", "卡特琳娜出手阻拦", "斯派克用药瓶设局", "封锁线前的选择"},
		[]string{"Tijuana asteroid colony", "ambushed at a bar", "before Katerina intervenes", "staged drug deal", "realizing they will never escape"}},
	{"stray-dog-strut", "67d38c6a49510ca98672d587c5bbaf40575167082a779708e67067105536ba8c", "chinatown-chase", "chase", "研究所如何用狗哨打乱追捕", "activate a dog whistle", 3,
		[]string{"哈基姆改变相貌", "手提箱在宠物店打开", "追踪项圈与狗哨", "斯派克放弃赏金救狗", "爱因留在比波普号"},
		[]string{"plastic surgery", "contains a Welsh Corgi", "collar with a tracking device", "lets Hakim go", "which he names"}},
}

type castRule struct {
	slug, name, action string
	ep, beat           int
}

var castRules = []castRule{
	{"spike", "斯派克", "接住了狗", 2, 4}, {"jet", "杰特", "追踪项圈", 2, 3}, {"asimov", "阿西莫夫", "把药卖掉", 1, 2}, {"katerina", "卡特琳娜", "封锁线", 1, 5}, {"ein", "爱因", "打开车门", 2, 5}, {"hakim", "阿卜杜勒·哈基姆", "改变了相貌", 2, 1},
}

type sceneRule struct{ name, anchor, block, chapter string }

var sceneRules = []sceneRule{
	{"药瓶赴约", "drug-deal", "episode-4", "asteroid-blues"}, {"宠物店开箱", "pet-shop", "episode-2", "stray-dog-strut"}, {"街头遛狗", "dog-walk", "episode-3", "stray-dog-strut"},
}

func eventSource(docs []researchDocument, claim researchClaim, needle string) (int, string) {
	for _, d := range docs {
		if d.ID != fmt.Sprintf("guide-%d", claim.Episode) {
			continue
		}
		for _, c := range claim.Cites {
			if c.ID == d.ID && c.Sentence > 0 && c.Sentence <= len(d.Sentences) && strings.Contains(d.Sentences[c.Sentence-1], needle) {
				return c.Sentence, c.URL
			}
		}
	}
	return 0, ""
}
func narrativeChecks(in scopeInput) []scopeCheck {
	checks := []scopeCheck{{Name: "人物"}, {Name: "剧情"}, {Name: "世界观"}, {Name: "时间线"}}
	valid := validateDocuments(in.Documents) == nil && validateClaims(in.Claims, in.Documents) == nil
	var events [2][]researchClaim
	for _, e := range in.Claims {
		if e.Episode == 1 || e.Episode == 2 {
			events[e.Episode-1] = append(events[e.Episode-1], e)
		}
	}
	var sources [2][5]string
	for i, rule := range episodeRules {
		var ch scopeChapter
		for _, c := range in.Chapters {
			if c.Episode == i+1 && c.Slug == rule.slug {
				ch = c
			}
		}
		chain := valid && len(events[i]) == len(rule.events)
		previous := 0
		for n, title := range rule.events {
			var pos int
			var source string
			if valid && n < len(events[i]) {
				pos, source = eventSource(in.Documents, events[i][n], rule.locators[n])
			}
			ordered := pos > previous
			if ordered {
				previous = pos
				sources[i][n] = source
			}
			chain = chain && ordered
			proof := fmt.Sprintf("本集事件 %d · 来源第 %d 句", n+1, pos)
			if n > 0 {
				proof = rule.events[n-1] + " → " + proof
			}
			checks[3].add(ordered, fmt.Sprintf("第 %d 集 · %s", i+1, title), proof, source, "缺少对应事件的同集取证，或先后次序与来源不一致")
		}
		anchor := false
		for _, p := range ch.Passages {
			if p.Block == rule.block && p.Text != "" {
				anchor = true
			}
		}
		var gaps []string
		if !chain {
			gaps = append(gaps, "缺关键事件及对应出处")
		}
		if !ch.Linked {
			gaps = append(gaps, "章节尚未关联故事线")
		}
		if !ch.Sourced {
			gaps = append(gaps, "正文引用的双来源快照不匹配")
		}
		if ch.Copy != rule.copy {
			gaps = append(gaps, "正文与已核实的五段导读版本不一致")
		}
		if ch.Scene != rule.scene || ch.Block != rule.block || !anchor {
			gaps = append(gaps, "缺少场景到原文段落的回跳")
		}
		plot := len(gaps) == 0
		checks[1].add(plot, fmt.Sprintf("第 %d 集 · 从悬赏到结局", i+1), "五段导读与双来源快照匹配，故事线、场景及正文段落已关联", "/work/"+in.World+"/chapter/"+rule.slug+"#passage-"+rule.scene, strings.Join(gaps, "；"))
		var source string
		if valid && len(events[i]) >= rule.settingBeat {
			_, source = eventSource(in.Documents, events[i][rule.settingBeat-1], rule.needle)
		}
		checks[2].add(source != "" && ch.Place != "" && ch.Scene == rule.scene && ch.Text != "" && in.Setting != "", rule.setting, ch.Place+" · "+rule.events[rule.settingBeat-1], source, "缺少设定影响行动的出处、场景或世界观说明")
	}
	for _, p := range castRules {
		c := in.People[p.slug]
		src := sources[p.ep-1][p.beat-1]
		var gaps []string
		if c.Name != p.name || !strings.Contains(c.Bio, p.action) {
			gaps = append(gaps, "人物介绍缺少本期核实的关键行动："+p.action)
		}
		if !slices.Contains(c.Episodes, p.ep) {
			gaps = append(gaps, fmt.Sprintf("未关联出场章节第 %d 集", p.ep))
		}
		if src == "" {
			gaps = append(gaps, "关键行动缺少同集来源")
		}
		checks[0].add(len(gaps) == 0, p.name, c.Bio, src, strings.Join(gaps, "；"))
	}
	return checks
}

var shaRE = regexp.MustCompile("^[a-f0-9]{64}$")

func currentArt(a scopeArt) bool {
	if !a.Verified || a.Credit == "" || !shaRE.MatchString(a.SHA) {
		return false
	}
	var path string
	switch {
	case strings.HasPrefix(a.Image, "/art/portraits/") && publicPortraitName.MatchString(strings.TrimPrefix(a.Image, "/art/portraits/")):
		path = filepath.Join("/media", filepath.Base(a.Image))
	case strings.HasPrefix(a.Image, "/art/bebop/") && a.Image == "/art/bebop/"+filepath.Base(a.Image):
		path = filepath.Join("/reader-art/bebop", filepath.Base(a.Image))
	default:
		return false
	}
	b, e := os.ReadFile(path)
	return e == nil && len(b) > 12 && string(b[:4]) == "RIFF" && string(b[8:12]) == "WEBP" && (!strings.HasPrefix(a.Image, "/art/portraits/") || filepath.Base(path) == digest(b)[7:]+".webp")
}
func sceneOriginal(candidate scopeImage, batch string) ([]byte, string, string) {
	g, r := candidate.Generated, candidate.Review
	if candidate.Stage != "passed" || candidate.Withdrawn != "" || r.Decision != "pass" || !shaRE.MatchString(candidate.Ref) || g.Reference_SHA != candidate.Ref || r.Model == "" || g.Model == "" {
		return nil, "", "缺少有效的生成、参考图或验图凭证"
	}
	for _, key := range []string{"identity", "scene", "anatomy", "clean", "crop"} {
		if !r.Checks[key] {
			return nil, "", "验图未通过：" + key
		}
	}
	phone := candidate.Crop
	if phone == "" {
		phone = g.Phone
	}
	base := fmt.Sprintf("%s-%d-r%d", batch, candidate.Position, candidate.Round)
	expectedCrop := base + "-crop0.jpg"
	if candidate.Crop != "" {
		expectedCrop = base + "-crop1.jpg"
	}
	if g.Image != base+".png" || phone != expectedCrop || phone != r.Phone {
		return nil, "", "验图样张与当前轮次不匹配"
	}
	b, err := os.ReadFile(filepath.Join("/media", g.Image))
	if err != nil || digest(b) != "sha256:"+g.SHA {
		return nil, "", "原图缺失或指纹不匹配"
	}
	cfg, e := png.DecodeConfig(bytes.NewReader(b))
	if e != nil || cfg.Width != 1536 || cfg.Height != 1024 {
		return nil, "", "原图尺寸异常"
	}
	crop, e := os.ReadFile(filepath.Join("/media", phone))
	if e != nil {
		return nil, "", "手机样张缺失"
	}
	cfg, e = jpeg.DecodeConfig(bytes.NewReader(crop))
	if e != nil || cfg.Width != 358 || cfg.Height != 225 {
		return nil, "", "手机样张尺寸异常"
	}
	return b, digest(crop)[7:], ""
}

// Content-address the actual public derivative. Readers can request only files
// referenced by a currently published scope; pending files have no public URL.
func saveScene(b []byte) (string, error) {
	im, e := png.Decode(bytes.NewReader(b))
	if e != nil {
		return "", e
	}
	var out bytes.Buffer
	if e = jpeg.Encode(&out, im, &jpeg.Options{Quality: 85}); e != nil {
		return "", e
	}
	key := digest(out.Bytes())
	path, e := archivePath(key)
	if e != nil {
		return "", e
	}
	if old, e := os.ReadFile(path); e != nil || digest(old) != key {
		if e = os.MkdirAll(filepath.Dir(path), 0700); e != nil {
			return "", e
		}
		f, e := os.CreateTemp(filepath.Dir(path), ".scene-")
		if e != nil {
			return "", e
		}
		defer os.Remove(f.Name())
		_, e = f.Write(out.Bytes())
		cerr := f.Close()
		if e != nil {
			return "", e
		}
		if cerr != nil {
			return "", cerr
		}
		if e = os.Rename(f.Name(), path); e != nil {
			return "", e
		}
	}
	return "/art/episode-scenes/" + key[7:] + ".jpg", nil
}
func imageChecks(in scopeInput) (scopeCheck, []scenePick, error) {
	return scopedImages(in, "bebop", castRules, sceneRules, "#bebop-media")
}
func scopedImages(in scopeInput, batch string, cast []castRule, scenes []sceneRule, source string) (scopeCheck, []scenePick, error) {
	check := scopeCheck{Name: "图片"}
	check.add(currentArt(in.Art[in.World]), "作品主视觉", "公开衍生图与已验图原稿的生成记录可追溯", "", "主视觉文件或生成验图记录缺失")
	for _, p := range cast {
		a := in.Art[p.slug]
		check.add(currentArt(a), p.name+"肖像", "角色列表及角色页共用已验图原稿的公开衍生图", "", "肖像文件或生成验图记录缺失")
	}
	return selectScenes(in, batch, scenes, source, check)
}
func selectScenes(in scopeInput, batch string, scenes []sceneRule, source string, check scopeCheck) (scopeCheck, []scenePick, error) {
	picks := []scenePick{}
	// Prefer an approved first rendering, then a crop without rework, then an
	// uncontested first assessment. Variant order is the deterministic tie break.
	sort.SliceStable(in.Images, func(i, j int) bool {
		a, b := in.Images[i], in.Images[j]
		if a.Round != b.Round {
			return a.Round < b.Round
		}
		if (a.Crop == "") != (b.Crop == "") {
			return a.Crop == ""
		}
		if (a.Review.SecondModel == "") != (b.Review.SecondModel == "") {
			return a.Review.SecondModel == ""
		}
		return a.Variant < b.Variant
	})
	for _, rule := range scenes {
		proof := ""
		var gaps []string
		for _, c := range in.Images {
			if c.Name != rule.name {
				continue
			}
			b, cropSHA, gap := sceneOriginal(c, batch)
			if gap != "" {
				gaps = append(gaps, fmt.Sprintf("候选 %d：%s", c.Variant, gap))
				continue
			}
			image, e := saveScene(b)
			if e != nil {
				return check, nil, e
			}
			picks = append(picks, scenePick{rule.name, rule.chapter, rule.block, rule.anchor, c.Slot, c.Round, c.Variant, image, c.Generated.SHA, c.Review.Phone, cropSHA})
			proof = fmt.Sprintf("候选 %d · 第 %d 轮 · %s", c.Variant, c.Round, c.Review.Model)
			break
		}
		if len(gaps) == 0 {
			gaps = []string{"尚无候选图"}
		}
		check.add(proof != "", rule.name, proof, source, strings.Join(gaps, "；"))
	}
	return check, picks, nil
}
func AcceptEpisodeScope(ctx context.Context, db *pgxpool.Pool) error {
	return pgx.BeginTxFunc(ctx, db, pgx.TxOptions{IsoLevel: pgx.Serializable}, func(tx pgx.Tx) error { return acceptScopeTx(ctx, tx) })
}
func acceptScopeTx(ctx context.Context, tx pgx.Tx) error {
	var locked bool
	if e := tx.QueryRow(ctx, "SELECT pg_try_advisory_xact_lock(82045121)").Scan(&locked); e != nil || !locked {
		return e
	}
	var raw []byte
	e := tx.QueryRow(ctx, acceptanceInputs, scopeTopic, "bebop", []string{"cowboy-bebop-tv", "spike", "jet", "ein", "asimov", "katerina", "hakim"}, 1, 2).Scan(&raw)
	if errors.Is(e, pgx.ErrNoRows) {
		return nil
	}
	if e != nil {
		return e
	}
	var in scopeInput
	if e = json.Unmarshal(raw, &in); e != nil {
		return e
	}
	checks := narrativeChecks(in)
	art, picks, e := imageChecks(in)
	if e != nil {
		return e
	}
	checks = append(checks, art)
	return recordScope(ctx, tx, in, raw, scopeTopic, 1, 2, scopeRule, checks, picks)
}
func recordScope(ctx context.Context, tx pgx.Tx, in scopeInput, raw []byte, topic string, first, last, rule int, checks []scopeCheck, picks []scenePick) error {
	passed := true
	for _, c := range checks {
		for _, item := range c.Items {
			if item.Gap != "" {
				passed = false
			}
		}
	}
	// Materialize the exact reader paragraphs and their original return anchors.
	type readerBlock struct {
		Key     string   `json:"key"`
		Text    string   `json:"text"`
		Anchors []string `json:"anchors"`
	}
	copy := map[string][]readerBlock{}
	if passed {
		for _, ch := range in.Chapters {
			for n, p := range ch.Passages {
				block := readerBlock{Key: fmt.Sprintf("episode-%d", n+1), Text: p.Text, Anchors: []string{}}
				if p.Block != "" {
					block.Key = p.Block
					if p.Block == ch.Block {
						block.Anchors = []string{"passage-" + ch.Scene}
					}
				}
				copy[ch.Slug] = append(copy[ch.Slug], block)
			}
		}
	}
	frozen, _ := json.Marshal(copy)
	report, _ := json.Marshal(checks)
	selected, _ := json.Marshal(picks)
	fingerprint := digest(bytes.Join([][]byte{raw, report, selected}, []byte{0}))
	var id int64
	e := tx.QueryRow(ctx, `WITH added AS (INSERT INTO scope_acceptances(topic,v,rule,fingerprint,passed,checks,picks,research_run,reader_copy,edition_slug,scope_first,scope_last)
   VALUES($1,$2,$3,$4,$5,$6,$7,nullif($8,'')::uuid,$9,$10,$11,$12) ON CONFLICT DO NOTHING RETURNING id)
   SELECT id FROM added UNION ALL SELECT id FROM scope_acceptances WHERE topic=$1 AND v=$2 AND rule=$3 AND fingerprint=$4 LIMIT 1`, topic, in.Version, rule, fingerprint, passed, report, selected, in.Research, frozen, in.World, first, last).Scan(&id)
	if e != nil {
		return e
	}
	_, e = tx.Exec(ctx, `INSERT INTO scope_heads(topic,latest,published) VALUES($1,$2,CASE WHEN $3 THEN $2::bigint END)
   ON CONFLICT(topic) DO UPDATE SET latest=excluded.latest,published=excluded.published WHERE scope_heads.latest IS DISTINCT FROM excluded.latest`, topic, id, passed)
	return e
}
