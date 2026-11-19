// Package billing keeps wallet mutations and fulfillment independent of provider retries.
package billing

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrConflict = errors.New("conflicting operation")
var ErrPending = errors.New("receipt not ready for settlement")
var ErrWorkerOffline = errors.New("image fulfillment temporarily unavailable")
var ErrFunds = errors.New("insufficient available balance")

// Taking the account row lock serializes credits, holds, charges, refunds and releases.
// Each later statement runs with a fresh READ COMMITTED snapshot.
func accountLock(ctx context.Context, tx pgx.Tx, id string) error {
	var locked string
	return tx.QueryRow(ctx, "SELECT id::text FROM billing_accounts WHERE id=$1 FOR UPDATE", id).Scan(&locked)
}

func Reserve(ctx context.Context, db *pgxpool.Pool, account, key, rate string, request PreviewRequest, automatic bool) (id string, err error) {
	payload, err := json.Marshal(request)
	if err != nil {
		return "", err
	}
	err = pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		if e := accountLock(ctx, tx, account); e != nil {
			return e
		}
		var same bool
		e := tx.QueryRow(ctx, "SELECT id::text,rate_id=$3 AND request=$4::jsonb FROM billing_orders WHERE account_id=$1 AND client_key=$2", account, key, rate, payload).Scan(&id, &same)
		if e == nil {
			if !same {
				return ErrConflict
			}
			return nil
		}
		if !errors.Is(e, pgx.ErrNoRows) {
			return e
		}
		var current string
		var funds, worker bool
		e = tx.QueryRow(ctx, `SELECT r.id::text,b.available_micros>=r.cap_micros,
 EXISTS(SELECT FROM billing_preview_workers w WHERE (w.endpoint,w.provider_account,w.model)=(r.endpoint,r.provider_account,r.model) AND w.available_until>now()) FROM billing_balances b JOIN billing_rates r ON r.currency=b.currency
   WHERE b.id=$1 AND r.kind='retail' AND r.effective_at<=now() ORDER BY r.effective_at DESC LIMIT 1`, account).Scan(&current, &funds, &worker)
		if e != nil {
			return e
		}
		if !strings.EqualFold(current, rate) {
			return ErrConflict
		}
		if automatic && !worker {
			return ErrWorkerOffline
		}
		if !funds {
			return ErrFunds
		}
		return tx.QueryRow(ctx, "INSERT INTO billing_orders(account_id,client_key,rate_id,request,automatic) VALUES($1,$2,$3,$4,$5) RETURNING id::text", account, key, rate, payload, automatic).Scan(&id)
	})
	return
}

func Credit(ctx context.Context, db *pgxpool.Pool, account, key, actor, currency string, amount int64) error {
	return pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		if e := accountLock(ctx, tx, account); e != nil {
			return e
		}
		var sameCurrency bool
		if e := tx.QueryRow(ctx, "SELECT currency=$2 FROM billing_accounts WHERE id=$1", account, currency).Scan(&sameCurrency); e != nil {
			return e
		}
		if !sameCurrency {
			return ErrConflict
		}
		tag, e := tx.Exec(ctx, `INSERT INTO billing_entries(account_id,kind,reference,amount_micros,actor) VALUES($1,'credit',$2,$3,$4) ON CONFLICT(account_id,kind,reference) DO NOTHING`, account, key, amount, actor)
		if e != nil || tag.RowsAffected() > 0 {
			return e
		}
		var same bool
		if e = tx.QueryRow(ctx, "SELECT amount_micros=$3 FROM billing_entries WHERE account_id=$1 AND kind='credit' AND reference=$2", account, key, amount).Scan(&same); e != nil {
			return e
		}
		if !same {
			return ErrConflict
		}
		return nil
	})
}

// Claim is called by the private preview worker before it calls any paid service.
func Claim(ctx context.Context, db *pgxpool.Pool, id, endpoint, account, model string) error {
	if account == "" {
		account = "default"
	}
	tag, err := db.Exec(ctx, `UPDATE billing_orders o SET status='running' FROM billing_rates r WHERE o.id=$1 AND o.rate_id=r.id
 AND o.status='reserved' AND r.endpoint=$2 AND r.provider_account=$3 AND r.model=$4`, id, endpoint, account, model)
	if err == nil && tag.RowsAffected() != 1 {
		return ErrConflict
	}
	return err
}

func Deliver(ctx context.Context, db *pgxpool.Pool, id, sha string) error {
	tag, err := db.Exec(ctx, `UPDATE billing_orders SET status='delivered',delivery_sha256=$2 WHERE id=$1 AND
 (status='running' OR (status='delivered' AND delivery_sha256=$2))`, id, sha)
	if err == nil && tag.RowsAffected() != 1 {
		return ErrConflict
	}
	return err
}

func Change(ctx context.Context, db *pgxpool.Pool, id, account, action, actor, reason string) error {
	return pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		var owner, state string
		if e := tx.QueryRow(ctx, "SELECT account_id::text FROM billing_orders WHERE id=$1", id).Scan(&owner); e != nil {
			return e
		}
		if account != "" && owner != account {
			return pgx.ErrNoRows
		}
		if e := accountLock(ctx, tx, owner); e != nil {
			return e
		}
		if e := tx.QueryRow(ctx, "SELECT status FROM billing_orders WHERE id=$1 FOR UPDATE", id).Scan(&state); e != nil {
			return e
		}
		switch action {
		case "cancel":
			if state == "void" {
				return nil
			}
			if state != "reserved" && (account != "" || (state != "running" && state != "delivered")) {
				return ErrConflict
			}
			_, e := tx.Exec(ctx, "UPDATE billing_orders SET status='void',finished_at=now(),reason=$2 WHERE id=$1", id, actor+": "+reason)
			return e
		case "refund":
			if state == "refunded" {
				return nil
			}
			if state != "settled" {
				return ErrConflict
			}
			if _, e := tx.Exec(ctx, `INSERT INTO billing_entries(account_id,order_id,reference,kind,amount_micros,actor)
   SELECT account_id,id,id::text,'refund',charge_micros,$2 FROM billing_orders WHERE id=$1`, id, actor); e != nil {
				return e
			}
			_, e := tx.Exec(ctx, "UPDATE billing_orders SET status='refunded',reason=$2 WHERE id=$1", id, actor+": "+reason)
			return e
		case "settle":
			if state == "settled" || state == "refunded" {
				return nil
			}
			if state != "delivered" {
				return ErrConflict
			}
			return settle(ctx, tx, id, actor)
		default:
			return ErrConflict
		}
	})
}

// An accepted single-image delivery may charge only one supplier receipt. Duplicate
// observations of that receipt are safe; ambiguous successful attempts require review.
func settle(ctx context.Context, tx pgx.Tx, id, actor string) error {
	var call string
	var amount *int64
	var units []byte
	rows, err := tx.Query(ctx, `WITH own AS (SELECT DISTINCT endpoint,provider_account,receipt_key FROM model_usage_receipts WHERE subject='order:'||$1::text)
 SELECT v.call_id::text,CASE WHEN price IS NOT NULL THEN least(price,r.cap_micros)::bigint END,
 jsonb_build_object('input',v.input_tokens,'output',v.output_tokens,'total',v.total_tokens,'images',v.images,
 'cached',v.cached,'receipt',v.receipt_key,'uncapped_micros',price)
 FROM own JOIN billing_receipts v USING(endpoint,provider_account,receipt_key)
 JOIN billing_orders o ON o.id=$1::uuid JOIN billing_rates r ON r.id=o.rate_id
 CROSS JOIN LATERAL (SELECT billing_amount(r,v.meter_state,v.total_tokens,v.input_tokens,v.output_tokens,v.images,v.cached) price) q
 WHERE v.successful AND v.images=1 AND v.subject='order:'||$1::text
 AND NOT EXISTS (SELECT FROM billing_receipts x JOIN own k USING(endpoint,provider_account,receipt_key)
 WHERE x.multiple_subjects OR x.subject<>'order:'||$1::text OR x.meter_state='inconsistent' OR x.unfinished)`, id)
	if err != nil {
		return err
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		if err = rows.Scan(&call, &amount, &units); err != nil {
			return err
		}
		count++
	}
	if err = rows.Err(); err != nil {
		return err
	}
	if count != 1 || amount == nil {
		return ErrPending
	}
	if _, err = tx.Exec(ctx, `UPDATE billing_orders SET status='settled',receipt_id=$2,charge_micros=$3,units=$4,finished_at=now() WHERE id=$1`, id, call, *amount, units); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO billing_entries(account_id,order_id,reference,kind,amount_micros,actor)
 SELECT account_id,id,id::text,'charge',-charge_micros,$2 FROM billing_orders WHERE id=$1`, id, actor)
	return err
}
