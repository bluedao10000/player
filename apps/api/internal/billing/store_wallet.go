package billing

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

const walletProduct = storeBundle + ".commission.20"
const walletCreditMicros int64 = 20_000_000

// Apple signs the wallet UUID as the appAccountToken. Both notification and
// device delivery credit that wallet, including after logout or app restart.
func recordWallet(ctx context.Context, db *pgxpool.Pool, t storeTransaction, reverse bool) error {
	if !t.validEnvelope() || t.ProductID != walletProduct || t.Type != "Consumable" || (reverse && t.RevocationDate != 0) {
		return ErrConflict
	}
	return pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		var currency string
		if e := tx.QueryRow(ctx, "SELECT currency FROM billing_accounts WHERE id=$1 FOR UPDATE", t.AppAccountToken).Scan(&currency); e != nil {
			return e
		}
		if currency != "CNY" {
			return ErrConflict
		}
		_, e := tx.Exec(ctx, `INSERT INTO billing_store_topups(transaction_id,account_id,environment,bundle_id,purchased_at,currency,price_milli)
          VALUES ($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(transaction_id) DO NOTHING`, t.TransactionID, t.AppAccountToken, t.Environment, t.BundleID, time.UnixMilli(t.PurchaseDate), t.Currency, t.Price)
		if e != nil {
			return e
		}
		var credited, matches bool
		var lastEvent *time.Time
		if e = tx.QueryRow(ctx, `SELECT credited,refund_event_at,(account_id=$2 AND environment=$3 AND bundle_id=$4)
          FROM billing_store_topups WHERE transaction_id=$1 FOR UPDATE`, t.TransactionID, t.AppAccountToken, t.Environment, t.BundleID).Scan(&credited, &lastEvent, &matches); e != nil {
			return e
		}
		if !matches {
			return ErrConflict
		}
		var event *time.Time
		if reverse || t.RevocationDate > 0 {
			stamp := time.UnixMilli(t.SignedDate)
			if lastEvent != nil && !stamp.After(*lastEvent) {
				return nil
			}
			event = &stamp
		} else if lastEvent != nil {
			return nil
		}
		desired := t.RevocationDate == 0
		if desired != credited {
			kind, amount := "store_credit", walletCreditMicros
			if !desired {
				kind, amount = "store_refund", -walletCreditMicros
			}
			ref := fmt.Sprintf("apple:%s:%d", t.TransactionID, t.SignedDate)
			if _, e = tx.Exec(ctx, `INSERT INTO billing_entries(account_id,kind,amount_micros,reference,actor)
              VALUES($1,$2,$3,$4,'app-store')`, t.AppAccountToken, kind, amount, ref); e != nil {
				return e
			}
		}
		_, e = tx.Exec(ctx, "UPDATE billing_store_topups SET credited=$2,refund_event_at=coalesce($3,refund_event_at) WHERE transaction_id=$1", t.TransactionID, desired, event)
		return e
	})
}

func StoreWalletReceipt(db *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		var in struct{ SignedTransaction string }
		var t storeTransaction
		if c.ShouldBindJSON(&in) != nil || verifyStoreJWS(in.SignedTransaction, &t, &t.SignedDate) != nil {
			c.JSON(400, gin.H{"error": "App Store 充值验签失败。"})
			return
		}
		storeReply(c, recordWallet(c.Request.Context(), db, t, false))
	}
}
