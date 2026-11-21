package reader

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"os"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	queries "anime-agent/api/internal/repository/queries/reader"
)

const appleIssuer = "https://appleid.apple.com"

var appleKeys = oidc.NewRemoteKeySet(oidc.ClientContext(context.Background(), &http.Client{Timeout: 8 * time.Second}), appleIssuer+"/auth/keys")
var errNativeProof = errors.New("expired proof")

func (s service) appleChallenge(c *gin.Context) {
	nonce, err := randomToken()
	if err == nil {
		ctx := c.Request.Context()
		err = pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
			if _, e := tx.Exec(ctx, queries.PurgeChallenges); e != nil {
				return e
			}
			_, e := tx.Exec(ctx, queries.CreateChallenge, nonce)
			return e
		})
	}
	if err != nil {
		unavailable(c)
		return
	}
	c.JSON(200, gin.H{"nonce": nonce})
}

func (s service) nativeApple(c *gin.Context) {
	var in struct{ Token, Nonce, Bundle string }
	if c.ShouldBindJSON(&in) != nil || len(in.Token) > 16384 || len(in.Nonce) != 64 ||
		(in.Bundle != "com.kmszz.starWiki" && in.Bundle != "com.kmszz.starWiki.test" && in.Bundle != "com.kmszz.starWiki.dev") {
		fail(c, 400, "Apple 授权请求无效。")
		return
	}
	ctx := c.Request.Context()
	verifier := oidc.NewVerifier(appleIssuer, appleKeys, &oidc.Config{ClientID: in.Bundle, SupportedSigningAlgs: []string{oidc.RS256}})
	id, err := verifier.Verify(ctx, in.Token)
	hash := sha256.Sum256([]byte(in.Nonce))
	if err != nil || id.Subject == "" || id.Nonce != hex.EncodeToString(hash[:]) {
		fail(c, 401, "Apple 授权验证失败。")
		return
	}
	s.finishNative(c, func(tx pgx.Tx, owner string) (string, error) {
		tag, e := tx.Exec(ctx, queries.ConsumeChallenge, in.Nonce)
		if e != nil {
			return "", e
		}
		if tag.RowsAffected() != 1 {
			return "", errNativeProof
		}
		if _, e = tx.Exec(ctx, queries.LockAppleSubject, id.Subject); e != nil {
			return "", e
		}
		var existing string
		e = tx.QueryRow(ctx, queries.FindAppleReader, id.Subject).Scan(&existing)
		if e == nil {
			if owner != "" && existing != owner {
				return "", errLinked
			}
			return existing, nil
		}
		if !errors.Is(e, pgx.ErrNoRows) {
			return "", e
		}
		if owner == "" {
			if e = tx.QueryRow(ctx, queries.CreateIdentity).Scan(&owner); e != nil {
				return "", e
			}
		}
		_, e = tx.Exec(ctx, queries.LinkApple, id.Subject, owner)
		return owner, e
	})
}

func (s service) nativeWechat(c *gin.Context) {
	app, secret := os.Getenv("WECHAT_IOS_APP_ID"), os.Getenv("WECHAT_IOS_APP_SECRET")
	if app == "" || secret == "" {
		fail(c, 503, "微信移动应用尚未配置。")
		return
	}
	var in struct{ Code string }
	if c.ShouldBindJSON(&in) != nil || in.Code == "" || len(in.Code) > 512 {
		fail(c, 400, "微信授权码无效。")
		return
	}
	ctx := c.Request.Context()
	sub, union, err := exchangeWechat(ctx, &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, app, secret, in.Code)
	if err != nil {
		fail(c, 401, "微信授权失败，请重试。")
		return
	}
	s.finishNative(c, func(tx pgx.Tx, owner string) (string, error) { return verifiedWechat(ctx, tx, app, sub, union, owner) })
}

func verifiedWechat(ctx context.Context, tx pgx.Tx, app, sub, union, owner string) (string, error) {
	if union == "" {
		return wechatIdentity(ctx, tx, app, sub, owner)
	}
	if _, err := tx.Exec(ctx, queries.LockWechatUnion, union); err != nil {
		return "", err
	}
	var known string
	err := tx.QueryRow(ctx, queries.FindWechatUnion, union).Scan(&known)
	if err == nil {
		if owner != "" && known != owner {
			return "", errLinked
		}
		owner = known
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	id, err := wechatIdentity(ctx, tx, app, sub, owner)
	if err == nil && known == "" {
		_, err = tx.Exec(ctx, queries.LinkWechatUnion, union, id)
	}
	return id, err
}

func (s service) finishNative(c *gin.Context, identify func(pgx.Tx, string) (string, error)) {
	ctx := c.Request.Context()
	owner := c.GetString("reader")
	var token string
	err := pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		id, e := identify(tx, owner)
		if e != nil || owner != "" {
			return e
		}
		token, e = newSession(ctx, tx, id)
		return e
	})
	var pgerr *pgconn.PgError
	if errors.Is(err, errLinked) || (errors.As(err, &pgerr) && pgerr.Code == "23505") {
		fail(c, 409, "登录方式已绑定其他账户。")
		return
	}
	if errors.Is(err, errNativeProof) {
		fail(c, 401, "授权已过期，请重新登录。")
		return
	}
	if err != nil {
		unavailable(c)
		return
	}
	c.JSON(200, gin.H{"token": token})
}
