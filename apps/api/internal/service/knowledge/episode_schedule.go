package knowledge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	queries "anime-agent/api/internal/repository/queries/knowledge"
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
		if _, err := tx.Exec(ctx, queries.LockPlanRegistration); err != nil {
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
		if err := tx.QueryRow(ctx, queries.MatchEdition,
			w.World, w.Provider, w.ExternalID, kind, w.Year).Scan(&matched); err != nil {
			return err
		}
		if !matched {
			return errors.New("登记的来源版本与已有作品不匹配")
		}
		p := w.Automation
		_, err := tx.Exec(ctx, queries.SavePolicy,
			w.World, p.First, p.Last, p.Priority, p.MaxEpisodes, p.TokenBudget, p.ReserveTokens)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, queries.PauseUnadmittedTopics, w.World)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, queries.SaveManifest, w.World, digest(raw))
		return err
	})
}

func AdmitEpisode(ctx context.Context, db *pgxpool.Pool) error {
	return pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, queries.LockPlanRegistration); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, queries.AdmitEpisode)
		return err
	})
}

func ClaimCapture(ctx context.Context, db *pgxpool.Pool, out io.Writer) error {
	var job []byte
	err := pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, queries.LockCaptureQueue); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, queries.ExpireCaptureLeases); err != nil {
			return err
		}
		return tx.QueryRow(ctx, queries.ClaimCaptureLease).Scan(&job)
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
