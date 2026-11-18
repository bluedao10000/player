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
	seen := map[string]bool{}
	for _, d := range docs {
		prefix, _, _ := strings.Cut(d.ID, "-")
		urls := map[string]string{"official": "https://www.cowboy-bebop.net/story/", "guide": fmt.Sprintf("https://en.wikipedia.org/wiki/List_of_Cowboy_Bebop_episodes#ep%d", d.Episode)}
		if plan != nil {
			urls = map[string]string{"official": plan.Official, "guide": plan.Guide}
		}
		if d.Episode < first || d.Episode > last || urls[prefix] == "" || d.URL != urls[prefix] || seen[d.ID] || d.ID != fmt.Sprintf("%s-%d", prefix, d.Episode) || d.Locator == "" || d.Fetched.IsZero() || len(d.Text) < 60 || digest([]byte(d.Text)) != "sha256:"+d.SHA {
			return fmt.Errorf("分集来源或快照指纹核验失败")
		}
		if len(d.Sentences) == 0 || len(d.Sentences) > 100 {
			return fmt.Errorf("缺少快照句子索引")
		}
		for _, sentence := range d.Sentences {
			if strings.TrimSpace(sentence) == "" || !strings.Contains(d.Text, sentence) {
				return fmt.Errorf("来源句子与快照不一致")
			}
		}
		seen[d.ID] = true
	}
	if len(seen) != 2*(last-first+1) {
		return fmt.Errorf("每集需两份可核实的分集来源")
	}
	return nil
}

func validateClaims(claims []researchClaim, docs []researchDocument) error {
	return validateEpisodeClaims(claims, docs, 1, 2)
}
func validateEpisodeClaims(claims []researchClaim, docs []researchDocument, first, last int) error {
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
		if counts[ep] < 4 || counts[ep] > 6 || len(coverage[ep]) != 5 {
			return fmt.Errorf("第 %d 集的动机、转折、因果或双来源不完整", ep)
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
	res, err := (&http.Client{Timeout: 4 * time.Minute}).Do(req)
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
	if err = conn.QueryRow(ctx, "SELECT pg_try_advisory_lock(82045119)").Scan(&locked); err != nil || !locked {
		return err
	}
	defer conn.Exec(context.Background(), "SELECT pg_advisory_unlock(82045119)")
	var topic string
	var v int64
	var attempt, policy, first, last int
	var catalog []byte
	err = conn.QueryRow(ctx, `SELECT t.id,t.policy,w.version_id,coalesce(last.attempt,0),t.first_episode,t.last_episode,jsonb_build_object('edition',w.edition,'plan',p.spec,'revision_notes',(SELECT jsonb_agg(i->>'gap') FROM scope_heads h JOIN scope_acceptances a ON a.id=h.latest CROSS JOIN jsonb_array_elements(a.checks) c CROSS JOIN jsonb_array_elements(c->'items') i WHERE h.topic=t.id AND a.research_run IS NOT NULL AND c->>'name'<>'图片' AND i?'gap'),'first_episode',t.first_episode,'last_episode',t.last_episode,'new_cast',(SELECT people FROM episode_extensions WHERE topic=t.id),
 'characters',(SELECT jsonb_agg(jsonb_build_array(name,original_name,role) ORDER BY position) FROM work_characters WHERE v=w.version_id),
 'scope',(SELECT jsonb_agg(jsonb_build_array(episode,name,intro) ORDER BY episode) FROM work_chapters WHERE v=w.version_id AND episode BETWEEN t.first_episode AND t.last_episode))
 FROM research_topics t JOIN published_worlds w ON w.slug=t.world_slug
 LEFT JOIN episode_plans p ON p.topic=t.id
 LEFT JOIN LATERAL (SELECT * FROM research_runs WHERE topic=t.id AND policy=t.policy AND v=w.version_id ORDER BY attempt DESC LIMIT 1) last ON true
 WHERE t.enabled AND (p.id IS NOT NULL OR ((t.id,t.first_episode,t.last_episode) IN (('bebop-tv-1998-01-02',1,2),('bebop-tv-1998-03',3,3)))) AND (last.id IS NULL OR last.stage IN ('collect','write') OR
 (last.stage='failed' AND last.attempt<3 AND ($1 OR last.finished_at < now()-interval '5 minutes')))
 ORDER BY t.id LIMIT 1`, retryNow).Scan(&topic, &policy, &v, &attempt, &first, &last, &catalog)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	var id, phase string
	var stored []byte
	err = conn.QueryRow(ctx, `SELECT id::text,stage,documents FROM research_runs WHERE topic=$1 AND v=$2 AND policy=$3 AND stage IN ('collect','write')`, topic, v, policy).Scan(&id, &phase, &stored)
	if errors.Is(err, pgx.ErrNoRows) {
		phase = "collect"
		err = pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
			if e := tx.QueryRow(ctx, `INSERT INTO research_runs(topic,v,attempt,policy,stage) VALUES($1,$2,$3,$4,'collect') RETURNING id::text`, topic, v, attempt+1, policy).Scan(&id); e != nil {
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
			if _, e := tx.Exec(ctx, "UPDATE research_runs SET stage='failed',finished_at=now(),error=$2 WHERE id=$1", id, cause.Error()); e != nil {
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
			if _, e := tx.Exec(ctx, "UPDATE research_runs SET stage='write',model=$2,documents=$3 WHERE id=$1", id, result.Model, docs); e != nil {
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
	if err = validateEpisodeClaims(result.Claims, docs, first, last); err != nil {
		return fail(err)
	}
	return pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
		if _, e := tx.Exec(ctx, "UPDATE research_runs SET stage='ready',model=$2,claims=$3,finished_at=now() WHERE id=$1", id, result.Model, result.Claims); e != nil {
			return e
		}
		if selection.Plan != nil {
			if _, e := tx.Exec(ctx, enqueueEpisodeArt, topic); e != nil {
				return e
			}
		}
		if topic == "bebop-tv-1998-03" {
			_, err := tx.Exec(ctx, `INSERT INTO media_candidates(slot,batch,position,scene,variant,character_slug,stage) VALUES
			 (23,'bebop-e3',1,'赌桌上的筹码',1,NULL,'reference'),(24,'bebop-e3',2,'赌桌上的筹码',2,NULL,'reference'),(25,'bebop-e3',3,'赌桌上的筹码',3,NULL,'reference'),
			 (26,'bebop-portrait',5,'菲·瓦伦丁',1,'faye','reference'),(27,'bebop-portrait',6,'戈登',1,'gordon','reference') ON CONFLICT (batch,position) DO NOTHING`)
			if err != nil {
				return err
			}
		}
		return researchEvent(ctx, tx, id, fmt.Sprintf("%d 段导读逐段匹配到对应集的原文 · 可在 H5 样板预览", len(result.Claims)))
	})
}
func researchEvent(ctx context.Context, tx pgx.Tx, id, message string) error {
	_, err := tx.Exec(ctx, "INSERT INTO research_events(run_id,message) VALUES($1,$2)", id, message)
	return err
}
