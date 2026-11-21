package billing

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	domain "anime-agent/api/internal/service/billing"
)

func storeReply(c *gin.Context, err error) {
	if err == nil {
		c.Status(204)
		return
	}
	if errors.Is(err, domain.ErrConflict) || errors.Is(err, pgx.ErrNoRows) {
		c.JSON(409, gin.H{"error": "交易与当前读者或商品不匹配。"})
		return
	}
	fail(c, err)
}
func StoreReceipt(db *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		var in struct{ SignedTransaction string }
		var t domain.StoreTransaction
		if c.ShouldBindJSON(&in) != nil || domain.VerifyStoreJWS(in.SignedTransaction, &t, &t.SignedDate) != nil {
			c.JSON(400, gin.H{"error": "App Store 交易验签失败。"})
			return
		}
		storeReply(c, domain.RecordStore(c.Request.Context(), db, t, c.GetString("reader"), false))
	}
}
func StoreNotifications(db *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		var in struct{ SignedPayload string }
		var n domain.StoreNotification
		if c.ShouldBindJSON(&in) != nil || domain.VerifyStoreJWS(in.SignedPayload, &n, &n.SignedDate) != nil || !domain.AllowedStore(n.Data.BundleID, n.Data.Environment) || (n.Data.Environment == "Production" && n.Data.AppAppleID != 1496915312) {
			c.Status(400)
			return
		}
		switch n.NotificationType {
		case "REFUND", "REVOKE", "REFUND_REVERSED", "ONE_TIME_CHARGE":
		default:
			c.Status(204)
			return
		}
		var t domain.StoreTransaction
		if domain.VerifyStoreJWS(n.Data.SignedTransactionInfo, &t, &t.SignedDate) != nil || t.BundleID != n.Data.BundleID || t.Environment != n.Data.Environment {
			c.Status(400)
			return
		}
		if !domain.IsStorePlan(t.ProductID) {
			c.Status(204)
			return
		}
		if (n.NotificationType == "REFUND" || n.NotificationType == "REVOKE") && t.RevocationDate == 0 {
			c.Status(400)
			return
		}
		reverse := n.NotificationType == "REFUND_REVERSED"
		if reverse && t.RevocationDate != 0 {
			c.Status(400)
			return
		}
		if reverse || n.NotificationType == "REFUND" || n.NotificationType == "REVOKE" {
			t.SignedDate = n.SignedDate
		}
		storeReply(c, domain.RecordStore(c.Request.Context(), db, t, t.AppAccountToken, reverse))
	}
}
