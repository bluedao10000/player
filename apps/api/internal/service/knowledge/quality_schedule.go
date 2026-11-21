package knowledge

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	queries "anime-agent/api/internal/repository/queries/knowledge"
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
	if err = conn.QueryRow(ctx, queries.TryLockQuality).Scan(&acquired); err != nil || !acquired {
		return err
	}
	if _, err = conn.Exec(ctx, queries.FailInterruptedQuality); err != nil {
		return err
	}
	var id string
	err = conn.QueryRow(ctx, queries.BeginQualityIfDue).Scan(&id)
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
		_, err = conn.Exec(finish, queries.FailQuality, id, scanErr.Error())
		if err != nil {
			return err
		}
		return scanErr
	}
	_, err = conn.Exec(finish, queries.FinishQuality, id, report)
	return err
}
