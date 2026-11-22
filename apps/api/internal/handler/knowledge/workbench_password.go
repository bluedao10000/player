package knowledge

import (
	"net/http"
	"strings"

	queries "anime-agent/api/internal/repository/queries/knowledge"
	codes "anime-agent/api/internal/repository/queries/reader"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

func changeWorkbenchPassword(c *gin.Context, db *pgxpool.Pool) {
	var in struct {
		Code     string
		Password string
	}
	if !editorRequest(c, &in) {
		return
	}
	if len(in.Code) != 6 || strings.Trim(in.Code, "0123456789") != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入 6 位验证码。"})
		return
	}
	if len(in.Password) < 12 || len(in.Password) > 72 || strings.TrimSpace(in.Password) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "密码需为 12–72 字节。"})
		return
	}
	ctx := c.Request.Context()
	email := c.GetString("email")
	changed := false
	err := pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		key := "workbench:" + email
		var hash []byte
		err := tx.QueryRow(ctx, codes.AttemptCode, key).Scan(&hash)
		if err == pgx.ErrNoRows {
			return nil
		}
		if err != nil {
			return err
		}
		if bcrypt.CompareHashAndPassword(hash, []byte(in.Code)) != nil {
			return nil
		}
		digest, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, codes.ConsumeCode, key); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, queries.RegisterWorkbenchAccount, email); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, queries.SetWorkbenchPassword, email, digest); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, queries.RevokeWorkbenchSessions, email); err != nil {
			return err
		}
		changed = true
		return nil
	})
	if err != nil {
		c.Status(http.StatusServiceUnavailable)
	} else if !changed {
		c.JSON(http.StatusBadRequest, gin.H{"error": "验证码无效或已过期。"})
	} else {
		c.Status(http.StatusNoContent)
	}
}
