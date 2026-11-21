package reader

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/mail"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"anime-agent/api/internal/handler/billing"
	desk "anime-agent/api/internal/repository/queries/knowledge"
	queries "anime-agent/api/internal/repository/queries/reader"
	mailer "anime-agent/api/internal/service/reader"
)

type service struct{ db *pgxpool.Pool }

func Routes(r *gin.Engine, db *pgxpool.Pool) {
	s := service{db}
	g := r.Group("/api/v1/reader", func(c *gin.Context) {
		c.Header("Cache-Control", "private, no-store")
		c.Header("X-Robots-Tag", "noindex")
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 512<<10)
	})
	g.POST("/workbench/code", s.issue)
	g.POST("/workbench/verify", s.verify)
	g.POST("/code", s.issue)
	g.POST("/verify", s.verify)
	g.GET("/methods", func(c *gin.Context) { c.JSON(200, gin.H{"wechat": wechatEnabled()}) })
	g.POST("/wechat/start", s.startWechat)
	g.POST("/wechat/finish", s.finishWechat)
	g.GET("/native/challenge", s.appleChallenge)
	g.POST("/native/apple", s.nativeApple)
	g.POST("/native/wechat", s.nativeWechat)
	g.POST("/native/store/notifications", billing.StoreNotifications(db))
	g.Use(s.authenticate)
	g.POST("/native/store/receipt", billing.StoreReceipt(db))
	g.POST("/native/link/apple", s.nativeApple)
	g.POST("/native/link/wechat", s.nativeWechat)
	billing.PlanRoutes(g, db)
	g.POST("/wechat/link", s.startWechat)
	g.POST("/email/link", s.verify)
	g.GET("/state", s.state)
	g.POST("/logout", s.logout)
	g.POST("/merge", s.merge)
	g.POST("/bookmark", s.bookmark)
	g.POST("/progress", s.progress)
}

func fail(c *gin.Context, status int, msg string) { c.AbortWithStatusJSON(status, gin.H{"error": msg}) }
func unavailable(c *gin.Context)                  { fail(c, 503, "读者服务暂时不可用，请稍后重试。") }
func identity(c *gin.Context) (string, string, bool) {
	var in struct {
		Email string
		Code  string
	}
	if c.ShouldBindJSON(&in) != nil {
		fail(c, 400, "请输入邮箱。")
		return "", "", false
	}
	email := strings.ToLower(strings.TrimSpace(in.Email))
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email || len(email) > 254 || strings.ContainsAny(email, "\r\n") {
		fail(c, 400, "邮箱格式不正确。")
		return "", "", false
	}
	return email, in.Code, true
}

func (s service) issue(c *gin.Context) {
	email, _, ok := identity(c)
	if !ok {
		return
	}
	if !mailer.MailConfigured() {
		fail(c, 503, "邮件服务尚未配置。")
		return
	}
	ctx := c.Request.Context()
	key := email
	workbench := strings.HasPrefix(c.FullPath(), "/api/v1/reader/workbench/")
	if workbench {
		key = "workbench:" + email
	}
	number, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		unavailable(c)
		return
	}
	code := fmt.Sprintf("%06d", number.Int64())
	hash, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
	if err != nil {
		unavailable(c)
		return
	}
	limited := errors.New("cooldown")
	err = pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		// Serialize reservations so cooldowns and the shared mail budget hold across replicas.
		if _, err := tx.Exec(ctx, queries.LockCodeIssuer); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, queries.PurgeOldCodes); err != nil {
			return err
		}
		var busy bool
		if err := tx.QueryRow(ctx, queries.CodeQuotaReached).Scan(&busy); err != nil {
			return err
		}
		if busy {
			return limited
		}
		tag, err := tx.Exec(ctx, queries.IssueCode, key, hash)
		if err == nil && tag.RowsAffected() == 0 {
			return limited
		}
		return err
	})
	if errors.Is(err, limited) {
		c.Header("Retry-After", "60")
		fail(c, 429, "请稍后再获取验证码。")
		return
	}
	if err != nil {
		unavailable(c)
		return
	}
	if err = mailer.SendLoginCode(ctx, email, code, workbench); err != nil {
		_, _ = s.db.Exec(context.WithoutCancel(ctx), queries.InvalidateUndeliveredCode, key, hash)
		fail(c, 503, "邮件暂时无法送达，请稍后重试。")
		return
	}
	c.Status(204)
}

func (s service) verify(c *gin.Context) {
	email, code, ok := identity(c)
	if !ok {
		return
	}
	invalid := func() { fail(c, 400, "验证码无效或已过期，请检查或重新获取。") }
	if len(code) != 6 || strings.Trim(code, "0123456789") != "" {
		invalid()
		return
	}
	ctx := c.Request.Context()
	var token string
	workbench := strings.HasPrefix(c.FullPath(), "/api/v1/reader/workbench/")
	key := email
	if workbench {
		key = "workbench:" + email
	}
	verified := false
	err := pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		var hash []byte
		err := tx.QueryRow(ctx, queries.AttemptCode, key).Scan(&hash)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		if bcrypt.CompareHashAndPassword(hash, []byte(code)) != nil {
			return nil
		}
		if _, err = tx.Exec(ctx, queries.ConsumeCode, key); err != nil {
			return err
		}
		if workbench {
			if _, err = tx.Exec(ctx, desk.RegisterWorkbenchAccount, email); err != nil {
				return err
			}
			token, err = randomToken()
			if err != nil {
				return err
			}
			digest := sha256.Sum256([]byte(token))
			_, err = tx.Exec(ctx, desk.CreateEditorSession, digest[:], "email:"+email)
			verified = err == nil
			return err
		}
		var id string
		if owner := c.GetString("reader"); owner != "" {
			tag, err := tx.Exec(ctx, queries.BindEmail, owner, email)
			var conflict *pgconn.PgError
			if errors.As(err, &conflict) && conflict.Code == "23505" {
				return errLinked
			}
			if err != nil {
				return err
			}
			if tag.RowsAffected() == 0 {
				return errLinked
			}
			verified = true
			return nil
		}
		if err = tx.QueryRow(ctx, queries.FindOrCreateReader, email).Scan(&id); err != nil {
			return err
		}
		token, err = newSession(ctx, tx, id)
		verified = err == nil
		return err
	})
	if errors.Is(err, errLinked) {
		fail(c, 409, "该邮箱已属于其他账户，或当前账户已有邮箱。")
		return
	}
	if err != nil {
		unavailable(c)
		return
	}
	if !verified {
		invalid()
		return
	}
	if token == "" {
		c.Status(204)
		return
	}
	c.JSON(200, gin.H{"token": token})
}

func randomToken() (string, error) {
	raw := make([]byte, 32)
	_, err := rand.Read(raw)
	return hex.EncodeToString(raw), err
}

func newSession(ctx context.Context, tx pgx.Tx, id string) (string, error) {
	token, err := randomToken()
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256([]byte(token))
	if _, err = tx.Exec(ctx, queries.PurgeSessions); err != nil {
		return "", err
	}
	_, err = tx.Exec(ctx, queries.CreateSession, digest[:], id)
	return token, err
}

func (s service) authenticate(c *gin.Context) {
	token, ok := strings.CutPrefix(c.GetHeader("Authorization"), "Bearer ")
	if !ok || len(token) != 64 {
		fail(c, 401, "请先登录。")
		return
	}
	digest := sha256.Sum256([]byte(token))
	var id string
	err := s.db.QueryRow(c.Request.Context(), queries.AuthenticateSession, digest[:]).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		fail(c, 401, "会话已过期，请重新登录。")
		return
	}
	if err != nil {
		unavailable(c)
		return
	}
	c.Set("reader", id)
	c.Set("digest", digest[:])
	if c.Request.Method == "POST" && c.GetHeader("X-Reader-ID") != id {
		fail(c, 409, "读者账户已切换，请刷新。")
	}
}

func (s service) logout(c *gin.Context) {
	if _, err := s.db.Exec(c.Request.Context(), queries.RevokeSession, c.MustGet("digest")); err != nil {
		unavailable(c)
		return
	}
	c.Status(204)
}
