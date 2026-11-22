package reader

import (
	"crypto/sha256"
	"errors"
	"net/http"

	desk "anime-agent/api/internal/repository/queries/knowledge"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

func (s service) workbenchPasswordLogin(c *gin.Context) {
	var in struct{ Email, Password string }
	if c.ShouldBindJSON(&in) != nil {
		fail(c, http.StatusBadRequest, "请输入邮箱和密码。")
		return
	}
	email, ok := normalizedEmail(c, in.Email)
	password := in.Password
	if !ok {
		return
	}
	invalid := func() {
		fail(c, http.StatusUnauthorized, "邮箱或密码有误。仍可使用邮箱验证码登录。")
	}
	if len(password) < 12 || len(password) > 72 {
		invalid()
		return
	}
	ctx := c.Request.Context()
	var token string
	err := pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		var hash []byte
		err := tx.QueryRow(ctx, desk.AttemptWorkbenchPassword, email).Scan(&hash)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		if bcrypt.CompareHashAndPassword(hash, []byte(password)) != nil {
			return nil
		}
		created, err := randomToken()
		if err != nil {
			return err
		}
		digest := sha256.Sum256([]byte(created))
		if _, err = tx.Exec(ctx, desk.CreateEditorSession, digest[:], "email:"+email); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, desk.ResetWorkbenchPasswordAttempts, email); err != nil {
			return err
		}
		token = created
		return nil
	})
	if err != nil {
		unavailable(c)
		return
	}
	if token == "" {
		invalid()
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": token})
}
