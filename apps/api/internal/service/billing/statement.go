package billing

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	queries "anime-agent/database/queries/billing"
)

type Statement struct {
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
func ImportStatement(ctx context.Context, db *pgxpool.Pool, v Statement, actor string) error {
	day, err := time.Parse(time.DateOnly, v.Day)
	if err != nil {
		return ErrInvalidStatement
	}
	return pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, queries.LockStatementBucket, v.Day, v.Endpoint, v.Account, v.Model, v.Currency); err != nil {
			return err
		}
		var same bool
		err := tx.QueryRow(ctx, queries.MatchStatement, day, v.Endpoint, v.Account, v.Model, v.Currency, v.Ref, *v.Amount, v.Replaces).Scan(&same)
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
		err = tx.QueryRow(ctx, queries.LatestStatementReference, day, v.Endpoint, v.Account, v.Model, v.Currency).Scan(&latest)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if latest != v.Replaces {
			return ErrConflict
		}
		_, err = tx.Exec(ctx, queries.InsertStatement, day, v.Endpoint, v.Account, v.Model, v.Currency, *v.Amount, v.Ref, v.Replaces, actor)
		return err
	})
}

var ErrConflict = errors.New("conflicting operation")
var ErrInvalidStatement = errors.New("invalid statement day")
