package billing

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type statement struct {
	Day      string `json:"day" binding:"required"`
	Endpoint string `json:"endpoint" binding:"required,max=512"`
	Account  string `json:"provider_account" binding:"required,max=128"`
	Model    string `json:"model" binding:"required,max=200"`
	Currency string `json:"currency" binding:"required,len=3"`
	Amount   *int64 `json:"amount_micros" binding:"required,gte=0"`
	Ref      string `json:"reference" binding:"required,max=150"`
	Replaces string `json:"replaces" binding:"max=150"`
}

// A new reference may replace only the latest line from the same supplier bucket.
// The old line and original operator remain available for audit.
func importStatement(ctx context.Context, db *pgxpool.Pool, v statement, actor string) error {
	day, err := time.Parse(time.DateOnly, v.Day)
	if err != nil {
		return ErrInvalidStatement
	}
	return pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended(jsonb_build_array($1::text,$2::text,$3::text,$4::text,$5::text)::text,2630))`, v.Day, v.Endpoint, v.Account, v.Model, v.Currency); err != nil {
			return err
		}
		const bucket = " FROM billing_statements WHERE day=$1 AND endpoint=$2 AND provider_account=$3 AND model=$4 AND currency=$5"
		var same bool
		err := tx.QueryRow(ctx, "SELECT amount_micros=$7 AND replaces=$8"+bucket+" AND reference=$6", day, v.Endpoint, v.Account, v.Model, v.Currency, v.Ref, *v.Amount, v.Replaces).Scan(&same)
		if err == nil {
			if !same {
				return ErrConflict
			}
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		var latest string
		err = tx.QueryRow(ctx, "SELECT reference"+bucket+" ORDER BY id DESC LIMIT 1", day, v.Endpoint, v.Account, v.Model, v.Currency).Scan(&latest)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if latest != v.Replaces {
			return ErrConflict
		}
		_, err = tx.Exec(ctx, `INSERT INTO billing_statements(day,endpoint,provider_account,model,currency,amount_micros,reference,replaces,created_by)
    VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, day, v.Endpoint, v.Account, v.Model, v.Currency, *v.Amount, v.Ref, v.Replaces, actor)
		return err
	})
}

var ErrInvalidStatement = errors.New("invalid statement day")
