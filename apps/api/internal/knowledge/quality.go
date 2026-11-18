package knowledge

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed quality.sql
var qualitySQL string

func Quality(ctx context.Context, db *pgxpool.Pool, source, work string) (json.RawMessage, error) {
	var report json.RawMessage
	err := pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(82045102)"); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, "CREATE TEMP TABLE scope(work_id uuid PRIMARY KEY) ON COMMIT DROP"); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `INSERT INTO scope SELECT id FROM works w WHERE active AND ($1='all' OR EXISTS(SELECT FROM knowledge_source_records r JOIN knowledge_sources s ON s.id=r.source_id WHERE r.active AND r.work_id=w.id AND s.name=$1)) AND ($2='' OR id=nullif($2,'')::uuid)`, source, work)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return fmt.Errorf("no active works in scope")
		}
		if _, err = tx.Exec(ctx, qualitySQL); err != nil {
			return err
		}
		return tx.QueryRow(ctx, `SELECT jsonb_build_object('works',count(*),'mean_score',round(avg(score),2),'issues',coalesce((SELECT jsonb_object_agg(rule_id,n) FROM (SELECT rule_id,count(*) n FROM knowledge_quality_issues JOIN scope ON entity_id=work_id WHERE status='open' GROUP BY 1) a),'{}')) FROM knowledge_quality_scores JOIN scope USING(work_id)`).Scan(&report)
	})
	return report, err
}
