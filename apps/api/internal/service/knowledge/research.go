package knowledge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	queries "anime-agent/api/internal/repository/queries/knowledge"
)

type researchDocument struct {
	Sentences []string  `json:"sentences"`
	ID        string    `json:"id"`
	Episode   int       `json:"episode"`
	URL       string    `json:"url"`
	Locator   string    `json:"locator"`
	Text      string    `json:"text"`
	SHA       string    `json:"sha256"`
	Fetched   time.Time `json:"fetched_at"`
}
type researchCitation struct {
	Sentence int    `json:"sentence"`
	ID       string `json:"id"`
	Quote    string `json:"quote"`
	URL      string `json:"url"`
	Locator  string `json:"locator"`
}
type researchClaim struct {
	Episode int                `json:"episode"`
	Kind    string             `json:"kind"`
	Text    string             `json:"text"`
	Cites   []researchCitation `json:"cites"`
}
type researchResult struct {
	Model     string             `json:"model"`
	Documents []researchDocument `json:"documents"`
	Claims    []researchClaim    `json:"claims"`
}

func validateDocuments(docs []researchDocument) error {
	return validateEpisodeDocuments(docs, 1, 2)
}
func validateEpisodeDocuments(docs []researchDocument, first, last int) error {
	return validateSourceDocuments(docs, first, last, nil)
}
func validateSourceDocuments(docs []researchDocument, first, last int, plan *episodeSpec) error {
	policy, err := plan.evidence()
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, d := range docs {
		prefix, _, _ := strings.Cut(d.ID, "-")
		urls := map[string]string{"official": "https://www.cowboy-bebop.net/story/"}
		if plan != nil {
			urls["official"] = plan.Official
			if policy.Documents > 1 {
				urls["guide"] = plan.Guide
			}
		} else if policy.Documents > 1 {
			urls["guide"] = fmt.Sprintf("https://en.wikipedia.org/wiki/List_of_Cowboy_Bebop_episodes#ep%d", d.Episode)
		}
		want, allowed := urls[prefix]
		if !allowed || d.Episode < first || d.Episode > last || want == "" || d.URL != want || seen[d.ID] || d.ID != fmt.Sprintf("%s-%d", prefix, d.Episode) || d.Locator == "" || d.Fetched.IsZero() || len(d.Text) < 60 || digest([]byte(d.Text)) != "sha256:"+d.SHA {
			return fmt.Errorf("分集来源或快照指纹核验失败")
		}
		// 分集回顾动辄上百句；上限只用来挡住空索引和异常大的快照。
		if len(d.Sentences) == 0 || len(d.Sentences) > 200 {
			return fmt.Errorf("缺少快照句子索引")
		}
		for _, sentence := range d.Sentences {
			if strings.TrimSpace(sentence) == "" || !strings.Contains(d.Text, sentence) {
				return fmt.Errorf("来源句子与快照不一致")
			}
		}
		seen[d.ID] = true
	}
	if len(seen) != policy.Documents*(last-first+1) {
		return fmt.Errorf("每集需 %d 份可核实的分集来源", policy.Documents)
	}
	return nil
}

func validateClaims(claims []researchClaim, docs []researchDocument) error {
	return validateEpisodeClaims(claims, docs, 1, 2, pairedEvidence)
}
func validateEpisodeClaims(claims []researchClaim, docs []researchDocument, first, last int, policy sourceEvidence) error {
	index := map[string]researchDocument{}
	for _, d := range docs {
		index[d.ID] = d
	}
	counts := map[int]int{}
	coverage := map[int]map[string]bool{}
	for ep := first; ep <= last; ep++ {
		coverage[ep] = map[string]bool{}
	}
	for i := range claims {
		p := &claims[i]
		if p.Episode < first || p.Episode > last || (p.Kind != "motive" && p.Kind != "turn" && p.Kind != "cause") || utf8.RuneCountInString(p.Text) < 12 || utf8.RuneCountInString(p.Text) > 450 || len(p.Cites) == 0 || len(p.Cites) > 8 {
			return fmt.Errorf("段落 %d 不符合选题或缺少出处", i+1)
		}
		counts[p.Episode]++
		coverage[p.Episode][p.Kind] = true
		for j := range p.Cites {
			c := &p.Cites[j]
			d, ok := index[c.ID]
			if !ok || d.Episode != p.Episode || c.Sentence < 1 || c.Sentence > len(d.Sentences) {
				return fmt.Errorf("第 %d 段引用了不存在的分集或句号", i+1)
			}
			// The service resolves the model's pointer against the saved source.
			excerpt := []rune(d.Sentences[c.Sentence-1])
			if len(excerpt) > 100 {
				excerpt = excerpt[:100]
			}
			c.Quote = string(excerpt)
			c.URL = d.URL
			if !strings.Contains(c.URL, "#") {
				c.URL += "#"
			}
			c.URL += ":~:text=" + strings.ReplaceAll(url.QueryEscape(c.Quote), "+", "%20")
			c.Locator = d.Locator
			coverage[p.Episode][c.ID] = true
		}
	}
	for ep := first; ep <= last; ep++ {
		if counts[ep] < 4 || counts[ep] > 6 || len(coverage[ep]) != policy.Citations {
			return fmt.Errorf("第 %d 集的动机、转折、因果或来源引用不完整（需 %d 项）", ep, policy.Citations)
		}
	}
	return nil
}

func callResearch(ctx context.Context, base, id, phase string, catalog []byte) (researchResult, error) {
	var out researchResult
	req, err := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(base, "/")+"/internal/research/"+id+"/"+phase, bytes.NewReader(catalog))
	if err != nil {
		return out, err
	}
	req.Header.Set("Content-Type", "application/json")
	// One write request waits for both drafting and review in the durable session.
	res, err := (&http.Client{Timeout: 8 * time.Minute}).Do(req)
	if err != nil {
		return out, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 256<<10))
	if err != nil {
		return out, err
	}
	if res.StatusCode != 200 {
		var problem struct{ Error string }
		_ = json.Unmarshal(body, &problem)
		return out, fmt.Errorf("Pi %s HTTP %d %s", phase, res.StatusCode, problem.Error)
	}
	err = json.Unmarshal(body, &out)
	return out, err
}

// Each sweep owns a session advisory lock. A restarted worker resumes the same
// phase/request ID. Model or source failures get at most three recorded attempts.
func ResearchIfDue(ctx context.Context, db *pgxpool.Pool, retryNow bool) error {
	base := os.Getenv("KNOWLEDGE_RESEARCH_URL")
	if base == "" {
		return nil
	}
	pc, err := db.Acquire(ctx)
	if err != nil {
		return err
	}
	conn := pc.Conn()
	defer pc.Release()
	var locked bool
	if err = conn.QueryRow(ctx, queries.TryLockResearch).Scan(&locked); err != nil || !locked {
		return err
	}
	defer conn.Exec(context.Background(), queries.UnlockResearch)
	var topic string
	var v int64
	var attempt, policy, first, last int
	var catalog []byte
	err = conn.QueryRow(ctx, queries.NextResearchTopic, retryNow, os.Getenv("EPISODE_EXTERNAL_CAPTURE") == "true").Scan(&topic, &policy, &v, &attempt, &first, &last, &catalog)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	var id, phase string
	var stored []byte
	err = conn.QueryRow(ctx, queries.FindActiveRun, topic, v, policy).Scan(&id, &phase, &stored)
	if errors.Is(err, pgx.ErrNoRows) {
		phase = "collect"
		err = pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
			if e := tx.QueryRow(ctx, queries.StartResearch, topic, v, attempt+1, policy).Scan(&id); e != nil {
				return e
			}
			return researchEvent(ctx, tx, id, "选题已锁定 · Pi 开始联网核对官方资料与分集指南")
		})
	}
	if err != nil {
		return err
	}
	fail := func(cause error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		e := pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
			if _, e := tx.Exec(ctx, queries.FailResearch, id, cause.Error()); e != nil {
				return e
			}
			return researchEvent(ctx, tx, id, cause.Error())
		})
		return errors.Join(cause, e)
	}
	var selection struct{ Plan *episodeSpec }
	if err = json.Unmarshal(catalog, &selection); err != nil {
		return err
	}
	sourcePolicy, err := selection.Plan.evidence()
	if err != nil {
		return fail(err)
	}
	var docs []researchDocument
	if phase == "collect" {
		result, e := callResearch(ctx, base, id, "collect", catalog)
		if e != nil {
			return fail(e)
		}
		docs = result.Documents
		if e = validateSourceDocuments(docs, first, last, selection.Plan); e != nil {
			return fail(e)
		}
		err = pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
			if _, e := tx.Exec(ctx, queries.SaveEvidence, id, result.Model, docs); e != nil {
				return e
			}
			for _, d := range docs {
				if e := researchEvent(ctx, tx, id, "取得快照 · "+d.Locator); e != nil {
					return e
				}
			}
			return researchEvent(ctx, tx, id, "分集身份和快照指纹已核对 · Pi 开始串联动机、转折和后果")
		})
		if err != nil {
			return err
		}
	} else if err = json.Unmarshal(stored, &docs); err != nil {
		return fail(err)
	}
	result, err := callResearch(ctx, base, id, "write", catalog)
	if err != nil {
		return fail(err)
	}
	if err = validateEpisodeClaims(result.Claims, docs, first, last, sourcePolicy); err != nil {
		return fail(err)
	}
	return pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
		if _, e := tx.Exec(ctx, queries.FinishResearch, id, result.Model, result.Claims); e != nil {
			return e
		}
		if selection.Plan != nil {
			if _, e := tx.Exec(ctx, queries.EnqueueEpisodeArt, topic); e != nil {
				return e
			}
		}
		if topic == "bebop-tv-1998-03" {
			_, err := tx.Exec(ctx, queries.QueueBebopMedia)
			if err != nil {
				return err
			}
		}
		return researchEvent(ctx, tx, id, fmt.Sprintf("%d 段导读逐段匹配到对应集的原文 · 可在 H5 样板预览", len(result.Claims)))
	})
}
func researchEvent(ctx context.Context, tx pgx.Tx, id, message string) error {
	_, err := tx.Exec(ctx, queries.RecordResearchEvent, id, message)
	return err
}
