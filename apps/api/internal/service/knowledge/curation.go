package knowledge

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	catalog "anime-agent/api/internal/model/knowledge"
	queries "anime-agent/api/internal/repository/queries/knowledge"
)

var fields = map[string]string{"canonical_title": "title", "original_title": "original", "type": "type", "episodes": "episodes", "status": "status", "start_date": "start", "end_date": "end", "season": "season", "season_year": "year", "synopsis": "synopsis"}

func Decide(ctx context.Context, db *pgxpool.Pool, d catalog.Decision, operator, source string) error {
	if strings.TrimSpace(d.Reason) == "" || strings.TrimSpace(operator) == "" {
		return fmt.Errorf("operator and reason required")
	}
	return pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		if _, e := tx.Exec(ctx, queries.BeginCuration); e != nil {
			return e
		}
		var old, next json.RawMessage
		var err error
		switch d.Action {
		case "override", "undo":
			key, ok := fields[d.Field]
			if !ok || !catalog.ValidUUID(d.Work) {
				return fmt.Errorf("invalid field or work")
			}
			err = tx.QueryRow(ctx, queries.ReadActiveField, d.Work, d.Field).Scan(&old)
			if err != nil {
				return err
			}
			if d.Action == "undo" {
				tag, e := tx.Exec(ctx, queries.DeleteOverride, d.Work, key)
				if e != nil {
					return e
				}
				if tag.RowsAffected() == 0 {
					return fmt.Errorf("no override")
				}
			} else {
				var v any
				if json.Unmarshal(d.Value, &v) != nil {
					return fmt.Errorf("invalid scalar")
				}
				switch v.(type) {
				case string:
					if key == "episodes" || key == "year" {
						return fmt.Errorf("integer required")
					}
				case float64:
					if key != "episodes" && key != "year" {
						return fmt.Errorf("text required")
					}
				default:
					return fmt.Errorf("scalar required")
				}
				if key == "title" && strings.TrimSpace(v.(string)) == "" {
					return fmt.Errorf("empty title")
				}
				_, err = tx.Exec(ctx, queries.UpsertOverride, d.Work, key, d.Value)
			}
			if err == nil {
				_, err = tx.Exec(ctx, queries.MarkAffectedWork, d.Work)
			}
		case "approve", "reject", "merge", "split", "rematch":
			var from, to string
			if d.Action == "split" || d.Action == "rematch" {
				err = tx.QueryRow(ctx, queries.ActiveRecordWork, d.Record).Scan(&from)
				to = from
			} else {
				err = tx.QueryRow(ctx, queries.MatchPair, d.Candidate).Scan(&d.Record, &from, &to)
			}
			if err != nil {
				return err
			}
			restore := d.Work
			d.Work = from
			d.Field = "identity"
			if d.Action == "split" && restore != "" {
				err = tx.QueryRow(ctx, queries.LastCurationSource, restore, from, d.Record).Scan(&to)
				if err != nil {
					return err
				}
			} else if d.Action == "split" {
				err = tx.QueryRow(ctx, queries.CloneWork, from).Scan(&to)
				if err != nil {
					return err
				}
			}
			_, err = tx.Exec(ctx, queries.MarkAffectedWorks, []string{from, to})
			if err != nil {
				return err
			}
			var ids []int64
			err = tx.QueryRow(ctx, queries.FindDecisionRecords, d.Record, d.Action, from).Scan(&ids)
			if err != nil {
				return err
			}
			old, _ = json.Marshal(map[string]any{"work_id": from, "records": ids, "candidate_id": d.Candidate})
			switch d.Action {
			case "reject":
				_, err = tx.Exec(ctx, queries.RejectMatch, d.Record, to)
				if err == nil {
					_, err = tx.Exec(ctx, queries.DeleteMatch, d.Record, to)
				}
			case "rematch":
				_, err = tx.Exec(ctx, queries.DeleteIdentityPin, d.Record)
				if err == nil {
					_, err = tx.Exec(ctx, queries.ClearRejections, d.Record)
				}
			default:
				_, err = tx.Exec(ctx, queries.PinRecords, ids, to)
				if err == nil {
					_, err = tx.Exec(ctx, queries.MoveRecords, ids, to)
				}
			}
			next, _ = json.Marshal(map[string]any{"work_id": to, "records": ids, "decision": d.Action})
		default:
			return fmt.Errorf("unknown action")
		}
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, queries.Projection); err != nil {
			return err
		}
		if d.Action == "override" || d.Action == "undo" {
			err = tx.QueryRow(ctx, queries.ReadWorkField, d.Work, d.Field).Scan(&next)
			if err != nil {
				return err
			}
		}
		_, err = tx.Exec(ctx, queries.RecordDecision, d.Work, operator, d.Action, d.Field, old, next, d.Reason, source)
		return err
	})
}
