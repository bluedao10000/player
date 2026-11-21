package reader

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	queries "anime-agent/api/internal/repository/queries/reader"
)

const callbackPath = "/reader-auth/callback/wechat"

func wechatConfig() (id, secret, callback string) {
	return os.Getenv("WECHAT_APP_ID"), os.Getenv("WECHAT_APP_SECRET"), os.Getenv("WECHAT_CALLBACK_URL")
}
func wechatEnabled() bool {
	id, secret, cb := wechatConfig()
	u, err := url.Parse(cb)
	return id != "" && secret != "" && err == nil && u.Scheme == "https" && u.Host != "" && u.User == nil && u.Path == callbackPath && u.RawQuery == "" && u.Fragment == ""
}

func (s service) startWechat(c *gin.Context) {
	if !wechatEnabled() {
		fail(c, 503, "微信登录尚未配置。")
		return
	}
	var in struct{ ReturnTo string }
	if c.ShouldBindJSON(&in) != nil {
		fail(c, 400, "无效的返回地址。")
		return
	}
	// The callback always returns to /login. The H5 route validates this continuation again.
	if !strings.HasPrefix(in.ReturnTo, "/") || strings.HasPrefix(in.ReturnTo, "//") || strings.ContainsAny(in.ReturnTo, "\\\r\n") || len(in.ReturnTo) > 2048 {
		in.ReturnTo = "/me"
	}
	state, err := randomToken()
	if err != nil {
		unavailable(c)
		return
	}
	proof, err := randomToken()
	if err != nil {
		unavailable(c)
		return
	}
	hash := sha256.Sum256([]byte(proof))
	session, _ := c.Get("digest")
	ctx := c.Request.Context()
	err = pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, queries.PurgeWechatFlows); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, queries.StartWechatFlow, state, hash[:], session, in.ReturnTo)
		return err
	})
	if err != nil {
		unavailable(c)
		return
	}
	id, _, cb := wechatConfig()
	q := url.Values{"appid": {id}, "redirect_uri": {cb}, "response_type": {"code"}, "scope": {"snsapi_login"}, "state": {state}}
	c.JSON(200, gin.H{"url": "https://open.weixin.qq.com/connect/qrconnect?" + q.Encode() + "#wechat_redirect", "proof": proof})
}

func exchangeWechat(ctx context.Context, client *http.Client, id, secret, code string) (string, string, error) {
	q := url.Values{"appid": {id}, "secret": {secret}, "code": {code}, "grant_type": {"authorization_code"}}
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.weixin.qq.com/sns/oauth2/access_token?"+q.Encode(), nil)
	if err != nil {
		return "", "", err
	}
	res, err := client.Do(req)
	if err != nil {
		return "", "", errors.New("wechat unavailable")
	}
	defer res.Body.Close()
	var body struct {
		UnionID string `json:"unionid"`
		OpenID  string `json:"openid"`
		Token   string `json:"access_token"`
		Error   int    `json:"errcode"`
	}
	if res.StatusCode != 200 || json.NewDecoder(io.LimitReader(res.Body, 16384)).Decode(&body) != nil || body.Error != 0 || body.OpenID == "" || len(body.OpenID) > 128 || body.Token == "" {
		return "", "", errors.New("wechat did not verify identity")
	}
	return body.OpenID, body.UnionID, nil
}

var errLinked = errors.New("identity belongs to another account")

func wechatIdentity(ctx context.Context, tx pgx.Tx, app, subject, owner string) (string, error) {
	// Serialize concurrent first logins for the same verified provider identity.
	if _, err := tx.Exec(ctx, queries.LockWechatIdentity, app+":"+subject); err != nil {
		return "", err
	}
	var existing string
	err := tx.QueryRow(ctx, queries.FindWechatReader, app, subject).Scan(&existing)
	if err == nil {
		if owner != "" && owner != existing {
			return "", errLinked
		}
		return existing, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	if owner == "" {
		if err = tx.QueryRow(ctx, queries.CreateIdentity).Scan(&owner); err != nil {
			return "", err
		}
	}
	_, err = tx.Exec(ctx, queries.LinkWechat, owner, app, subject)
	var pgerr *pgconn.PgError
	if errors.As(err, &pgerr) && pgerr.Code == "23505" {
		return "", errLinked
	}
	return owner, err
}

func (s service) finishWechat(c *gin.Context) {
	var in struct{ State, Proof, Code string }
	if !wechatEnabled() || c.ShouldBindJSON(&in) != nil || len(in.State) != 64 || len(in.Proof) != 64 || len(in.Code) > 512 {
		fail(c, 400, "请重新发起微信登录。")
		return
	}
	ctx := c.Request.Context()
	proof := sha256.Sum256([]byte(in.Proof))
	var dest string
	var bound []byte
	err := s.db.QueryRow(ctx, queries.ConsumeWechatFlow, in.State, proof[:]).Scan(&dest, &bound)
	if errors.Is(err, pgx.ErrNoRows) {
		fail(c, 400, "授权已过期，请重试。")
		return
	}
	if err != nil {
		unavailable(c)
		return
	}
	if in.Code == "" {
		fail(c, 400, "微信授权已取消。")
		return
	}
	app, secret, _ := wechatConfig()
	client := &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	subject, union, err := exchangeWechat(ctx, client, app, secret, in.Code)
	if err != nil {
		fail(c, 502, "微信授权未完成，请重试。")
		return
	}
	var token string
	err = pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		var owner string
		if bound != nil {
			bearer := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
			hash := sha256.Sum256([]byte(bearer))
			if err := tx.QueryRow(ctx, queries.CheckBindingSession, bound, hash[:]).Scan(&owner); err != nil {
				return err
			}
		}
		id, err := verifiedWechat(ctx, tx, app, subject, union, owner)
		if err != nil || bound != nil {
			return err
		}
		token, err = newSession(ctx, tx, id)
		return err
	})
	if errors.Is(err, errLinked) {
		fail(c, 409, "此微信或读者账户已有绑定。")
		return
	}
	if errors.Is(err, pgx.ErrNoRows) {
		fail(c, 401, "请登录原账户后再绑定。")
		return
	}
	if err != nil {
		unavailable(c)
		return
	}
	c.JSON(200, gin.H{"token": token, "returnTo": dest})
}
