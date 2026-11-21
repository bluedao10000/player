package knowledge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type episodePolicy struct {
	First, Last, Priority, MaxEpisodes, ReserveTokens int
	TokenBudget                                       int64
}

// Registration is one transaction. The Guangzhou dispatcher alone enables new
// managed topics after checking the latest inventory and reserving the budget.
func RegisterProductionPlan(ctx context.Context, db *pgxpool.Pool, input io.Reader) error {
	var w editionPlan
	raw, err := io.ReadAll(io.LimitReader(input, 4<<20))
	if err != nil {
		return err
	}
	if err = json.Unmarshal(raw, &w); err != nil {
		return err
	}
	if w.Automation == nil {
		return errors.New("缺少作品补集策略")
	}
	for _, data := range w.Episodes {
		var spec episodeSpec
		if err := json.Unmarshal(data, &spec); err != nil {
			return err
		}
		if spec.SourceRules == nil {
			return errors.New("自动补集需要声明来源解析规则")
		}
	}
	return pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(82045125)"); err != nil {
			return err
		}
		if len(w.Episodes) > 0 {
			if err := registerEdition(ctx, tx, w); err != nil {
				return err
			}
		}
		kind := w.Type
		if kind == "" {
			kind = "TV"
		}
		var matched bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT FROM work_worlds w JOIN works k ON k.id=w.work_id
   JOIN work_sources s ON s.work_id=k.id WHERE w.slug=$1 AND s.provider=$2 AND s.external_id=$3 AND k.type=$4 AND k.season_year=$5)`,
			w.World, w.Provider, w.ExternalID, kind, w.Year).Scan(&matched); err != nil {
			return err
		}
		if !matched {
			return errors.New("登记的来源版本与已有作品不匹配")
		}
		p := w.Automation
		_, err := tx.Exec(ctx, `INSERT INTO episode_policies VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(world_slug) DO UPDATE SET
   first_episode=excluded.first_episode,last_episode=excluded.last_episode,priority=excluded.priority,
   max_episodes=excluded.max_episodes,token_budget=excluded.token_budget,reserve_tokens=excluded.reserve_tokens`,
			w.World, p.First, p.Last, p.Priority, p.MaxEpisodes, p.TokenBudget, p.ReserveTokens)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE research_topics t SET enabled=false FROM episode_plans p WHERE t.id=p.topic AND p.world_slug=$1
    AND NOT EXISTS(SELECT FROM scope_heads WHERE topic=p.topic AND published IS NOT NULL)
    AND NOT EXISTS(SELECT FROM episode_admissions WHERE topic=p.topic)
    AND NOT EXISTS(SELECT FROM research_runs WHERE topic=p.topic)`, w.World)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO episode_plan_manifests VALUES($1,$2,now()) ON CONFLICT(world_slug) DO UPDATE SET checksum=excluded.checksum,loaded_at=now()`, w.World, digest(raw))
		return err
	})
}

func AdmitEpisode(ctx context.Context, db *pgxpool.Pool) error {
	return pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(82045125)"); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `WITH next AS (SELECT topic,reserve_tokens FROM episode_policy_queue WHERE status='待准入'
    ORDER BY priority DESC,number,world_slug LIMIT 1), admitted AS (
    INSERT INTO episode_admissions(topic,reserve_tokens) SELECT * FROM next ON CONFLICT DO NOTHING RETURNING topic)
    UPDATE research_topics SET enabled=true FROM admitted WHERE id=admitted.topic`)
		return err
	})
}

func ClaimCapture(ctx context.Context, db *pgxpool.Pool, out io.Writer) error {
	var job []byte
	err := pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(82045126)"); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE episode_capture_attempts SET finished_at=now(),error='采集租约到期' WHERE finished_at IS NULL AND started_at<now()-interval '10 minutes'`); err != nil {
			return err
		}
		return tx.QueryRow(ctx, `WITH next AS (SELECT p.* FROM episode_plans p JOIN research_topics t ON t.id=p.topic
    LEFT JOIN episode_policies pol ON pol.world_slug=p.world_slug LEFT JOIN scope_heads h ON h.topic=p.topic
    LEFT JOIN episode_source_captures c ON c.topic=p.topic
    WHERE t.enabled AND h.published IS NULL AND (c.topic IS NULL OR c.spec<>p.spec OR c.captured_at<now()-interval '12 hours')
    AND NOT EXISTS (SELECT FROM research_runs WHERE topic=p.topic AND stage='ready' AND policy=t.policy)
    AND NOT EXISTS(SELECT FROM episode_capture_attempts a WHERE a.topic=p.topic AND (a.finished_at IS NULL OR a.finished_at>now()-interval '15 minutes'))
    ORDER BY pol.priority DESC NULLS LAST,p.number,p.world_slug LIMIT 1), lease AS (
    INSERT INTO episode_capture_attempts(topic) SELECT topic FROM next RETURNING id)
    SELECT jsonb_build_object('lease',lease.id,'world',next.world_slug,'spec',next.spec) FROM next,lease`).Scan(&job)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, string(job))
	return err
}
