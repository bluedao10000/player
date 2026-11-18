package knowledge

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// QualityIfDue shares a session lock across workers. A new lock owner can mark
// any unfinished attempt interrupted, including after a process restart.
func QualityIfDue(ctx context.Context, db *pgxpool.Pool) error {
	conn, err := pgx.ConnectConfig(ctx, db.Config().ConnConfig)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	var acquired bool
	if err = conn.QueryRow(ctx, "SELECT pg_try_advisory_lock(82045105)").Scan(&acquired); err != nil || !acquired {
		return err
	}
	if _, err = conn.Exec(ctx, "UPDATE knowledge_quality_runs SET status='failed',finished_at=now(),error_message='worker interrupted' WHERE status='running'"); err != nil {
		return err
	}
	var id string
	err = conn.QueryRow(ctx, `INSERT INTO knowledge_quality_runs(status) SELECT 'running'
 WHERE EXISTS(SELECT FROM works WHERE active) AND NOT EXISTS(
 SELECT FROM (SELECT status,finished_at FROM knowledge_quality_runs ORDER BY started_at DESC LIMIT 1) last_run
 WHERE finished_at + CASE WHEN status='failed' THEN interval '15 minutes' ELSE interval '6 hours' END >now())
 RETURNING id::text`).Scan(&id)
	if err == pgx.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	report, scanErr := Quality(ctx, db, "all", "")
	finish, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if scanErr != nil {
		_, err = conn.Exec(finish, "UPDATE knowledge_quality_runs SET status='failed',finished_at=now(),error_message=left($2,2000) WHERE id=$1", id, scanErr.Error())
		if err != nil {
			return err
		}
		return scanErr
	}
	_, err = conn.Exec(finish, "UPDATE knowledge_quality_runs SET status='success',finished_at=now(),report=$2 WHERE id=$1", id, report)
	return err
}
