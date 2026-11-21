package reader

import (
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	model "anime-agent/api/internal/model/reader"
	library "anime-agent/api/internal/repository/reader"
	account "anime-agent/api/internal/service/reader"
	queries "anime-agent/api/internal/repository/queries/reader"
)

func (s service) state(c *gin.Context) {
	result, err := library.State(c.Request.Context(), s.db, c.GetString("reader"), os.Getenv("WECHAT_IOS_APP_ID"), os.Getenv("WECHAT_APP_ID"))
	if err != nil {
		unavailable(c)
		return
	}
	c.Data(200, "application/json", result)
}

func (s service) bookmark(c *gin.Context) {
	var in struct {
		model.Entry
		Saved *bool
	}
	if c.ShouldBindJSON(&in) != nil || !in.Valid() || in.Saved == nil {
		fail(c, 400, "无效的收藏。")
		return
	}
	ctx := c.Request.Context()
	id := c.GetString("reader")
	err := pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		if *in.Saved {
			in.RecordedAt = time.Now().UnixMilli()
			return library.SaveBookmark(ctx, tx, id, in.Entry)
		}
		_, err := tx.Exec(ctx, queries.DeleteBookmark, id, in.Href)
		return err
	})
	if err != nil {
		unavailable(c)
		return
	}
	c.Status(204)
}

func (s service) progress(c *gin.Context) {
	var e model.Entry
	if c.ShouldBindJSON(&e) != nil || !e.Valid() || e.RecordedAt == 0 {
		fail(c, 400, "无效的阅读位置。")
		return
	}
	ctx := c.Request.Context()
	if err := pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error { return library.SavePosition(ctx, tx, c.GetString("reader"), e) }); err != nil {
		unavailable(c)
		return
	}
	c.Status(204)
}

func (s service) merge(c *gin.Context) {
	var in struct {
		Choice               string
		Bookmarks, Positions []model.Entry
	}
	if c.ShouldBindJSON(&in) != nil || (in.Choice != "merge" && in.Choice != "skip") || len(in.Bookmarks) > 2000 || len(in.Positions) > 2000 {
		fail(c, 400, "无效的导入请求。")
		return
	}
	for _, rows := range [][]model.Entry{in.Bookmarks, in.Positions} {
		for _, e := range rows {
			if !e.Valid() {
				fail(c, 400, "本机记录格式有误。")
				return
			}
		}
	}
	ctx := c.Request.Context()
	id := c.GetString("reader")
	err := account.ImportLibrary(ctx, s.db, id, in.Choice, in.Bookmarks, in.Positions)
	if err != nil {
		unavailable(c)
		return
	}
	c.Status(204)
}
