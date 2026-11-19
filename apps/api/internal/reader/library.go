package reader

import (
	"context"
	"encoding/json"
	"regexp"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

var chapterPath = regexp.MustCompile("^/(story/[a-z0-9-]+|work/[a-z0-9-]+/chapter/[a-z0-9-]+)$")
var paragraph = regexp.MustCompile("^[a-zA-Z0-9_-]{0,160}$")

type entry struct {
	Href       string `json:"href"`
	Title      string `json:"title"`
	Work       string `json:"work"`
	RecordedAt int64  `json:"recordedAt"`
	Percent    int    `json:"percent"`
	Anchor     string `json:"anchor"`
	Excerpt    string `json:"excerpt"`
}

func (e entry) valid() bool {
	return len(e.Href) <= 600 && chapterPath.MatchString(e.Href) &&
		e.Title != "" && utf8.RuneCountInString(e.Title) <= 512 &&
		e.Work != "" && utf8.RuneCountInString(e.Work) <= 512 &&
		e.RecordedAt >= 0 && e.Percent >= 0 && e.Percent <= 100 &&
		paragraph.MatchString(e.Anchor) && utf8.RuneCountInString(e.Excerpt) <= 200
}

func (s service) state(c *gin.Context) {
	var result json.RawMessage
	err := s.db.QueryRow(c.Request.Context(), `SELECT jsonb_build_object('id',r.id,'email',r.email,'pending',r.import_choice IS NULL,
 'bookmarks',COALESCE((SELECT jsonb_agg(jsonb_build_object('href',href,'title',title,'work',work,'recordedAt',recorded_at) ORDER BY recorded_at DESC,href) FROM reader_bookmarks WHERE reader_id=r.id),'[]'),
 'positions',COALESCE((SELECT jsonb_agg(jsonb_build_object('href',href,'title',title,'work',work,'percent',percent,'anchor',anchor,'excerpt',excerpt,'recordedAt',recorded_at) ORDER BY recorded_at DESC,href) FROM reader_positions WHERE reader_id=r.id),'[]'))
 FROM reader_identities r WHERE r.id=$1`, c.GetString("reader")).Scan(&result)
	if err != nil {
		unavailable(c)
		return
	}
	c.Data(200, "application/json", result)
}

func savePosition(ctx context.Context, tx pgx.Tx, id string, e entry) error {
	_, err := tx.Exec(ctx, `INSERT INTO reader_positions(reader_id,href,title,work,percent,anchor,excerpt,recorded_at)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8)
 ON CONFLICT(reader_id,href) DO UPDATE SET title=excluded.title,work=excluded.work,percent=excluded.percent,anchor=excluded.anchor,excerpt=excluded.excerpt,recorded_at=excluded.recorded_at
 WHERE reader_positions.recorded_at<excluded.recorded_at`, id, e.Href, e.Title, e.Work, e.Percent, e.Anchor, e.Excerpt, min(e.RecordedAt, time.Now().UnixMilli()))
	return err
}

func saveBookmark(ctx context.Context, tx pgx.Tx, id string, e entry) error {
	_, err := tx.Exec(ctx, `INSERT INTO reader_bookmarks(reader_id,href,title,work,recorded_at) VALUES($1,$2,$3,$4,$5) ON CONFLICT(reader_id,href) DO NOTHING`, id, e.Href, e.Title, e.Work, min(e.RecordedAt, time.Now().UnixMilli()))
	return err
}

func (s service) bookmark(c *gin.Context) {
	var in struct {
		entry
		Saved *bool
	}
	if c.ShouldBindJSON(&in) != nil || !in.valid() || in.Saved == nil {
		fail(c, 400, "无效的收藏。")
		return
	}
	ctx := c.Request.Context()
	id := c.GetString("reader")
	err := pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		if *in.Saved {
			in.RecordedAt = time.Now().UnixMilli()
			return saveBookmark(ctx, tx, id, in.entry)
		}
		_, err := tx.Exec(ctx, "DELETE FROM reader_bookmarks WHERE reader_id=$1 AND href=$2", id, in.Href)
		return err
	})
	if err != nil {
		unavailable(c)
		return
	}
	c.Status(204)
}

func (s service) progress(c *gin.Context) {
	var e entry
	if c.ShouldBindJSON(&e) != nil || !e.valid() || e.RecordedAt == 0 {
		fail(c, 400, "无效的阅读位置。")
		return
	}
	ctx := c.Request.Context()
	if err := pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error { return savePosition(ctx, tx, c.GetString("reader"), e) }); err != nil {
		unavailable(c)
		return
	}
	c.Status(204)
}

func (s service) merge(c *gin.Context) {
	var in struct {
		Choice               string
		Bookmarks, Positions []entry
	}
	if c.ShouldBindJSON(&in) != nil || (in.Choice != "merge" && in.Choice != "skip") || len(in.Bookmarks) > 2000 || len(in.Positions) > 2000 {
		fail(c, 400, "无效的导入请求。")
		return
	}
	for _, rows := range [][]entry{in.Bookmarks, in.Positions} {
		for _, e := range rows {
			if !e.valid() {
				fail(c, 400, "本机记录格式有误。")
				return
			}
		}
	}
	ctx := c.Request.Context()
	id := c.GetString("reader")
	err := pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		// The first completed decision wins, including a simultaneous skip on another device.
		tag, err := tx.Exec(ctx, "UPDATE reader_identities SET import_choice=$2 WHERE id=$1 AND import_choice IS NULL", id, in.Choice)
		if err != nil || tag.RowsAffected() == 0 || in.Choice == "skip" {
			return err
		}
		for _, e := range in.Bookmarks {
			if err := saveBookmark(ctx, tx, id, e); err != nil {
				return err
			}
		}
		for _, e := range in.Positions {
			if err := savePosition(ctx, tx, id, e); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		unavailable(c)
		return
	}
	c.Status(204)
}
