package knowledge

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type editorScope struct{ World, Chapter, Name string }

var editorScopes = []editorScope{{"naruto-tv-2002", "enter-naruto", "火影忍者 · 第 1 集"}, {"tidepost", "night-courier", "潮汐邮局 · 海风里的夜班邮差"}}

type editBlock struct{ Key, Text string }
type editSnapshot struct {
	Chapter struct {
		Blocks   []editBlock
		Passages []editBlock `json:"reader_passages"`
		Research struct {
			Run string `json:"run_id"`
		}
	}
	Scenes map[string]struct{ Scene struct{ Text string } }
}

func (s editSnapshot) fields() map[string]string {
	out := map[string]string{}
	blocks := s.Chapter.Passages
	if blocks == nil {
		blocks = s.Chapter.Blocks
	}
	for _, b := range blocks {
		out["block/"+b.Key] = b.Text
	}
	for k, v := range s.Scenes {
		out["scene/"+k] = v.Scene.Text
	}
	return out
}

type editSource struct {
	ID      string `json:"id"`
	URI     string `json:"uri"`
	Locator string `json:"locator"`
	Text    string `json:"text"`
	Digest  string `json:"sha256"`
}
type editChange struct {
	Key    string `json:"key"`
	Text   string `json:"text"`
	Source string `json:"source"`
	Quote  string `json:"quote"`
}
type copyField struct {
	Original string `json:"original"`
	Text     string `json:"text"`
}
type editorJob struct {
	ID, World, Chapter, Instruction, Feedback, Issue, Status, Lease string
	Round, Attempt, Retries                                         int
	Base                                                            editSnapshot
	Sources                                                         []editSource
	Draft                                                           []editChange
}

func editorEvent(ctx context.Context, tx pgx.Tx, id, stage, note string, detail any) error {
	_, err := tx.Exec(ctx, `INSERT INTO editor_events(task,round,attempt,stage,note,detail) SELECT id,round,attempt,$2,$3,$4 FROM editor_tasks WHERE id=$1`, id, stage, note, detail)
	return err
}
func editorRequest(c *gin.Context, v any) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 12000)
	if err := c.ShouldBindJSON(v); err != nil {
		c.JSON(400, gin.H{"error": "请求内容无效"})
		return false
	}
	return true
}

func editorRoutes(r *gin.Engine, admin *gin.RouterGroup, db *pgxpool.Pool) {
	admin.POST("/editor/session", func(c *gin.Context) {
		var b [32]byte
		rand.Read(b[:])
		token := hex.EncodeToString(b[:])
		hash := sha256.Sum256([]byte(token))
		_, err := db.Exec(c.Request.Context(), `WITH clean AS(DELETE FROM editor_sessions WHERE expires_at<=now()) INSERT INTO editor_sessions(hash,operator) VALUES($1,$2)`, hash[:], c.GetString("operator"))
		if err != nil {
			c.Status(503)
			return
		}
		c.JSON(200, gin.H{"token": token})
	})
	g := r.Group("/api/v1/editor", func(c *gin.Context) {
		c.Header("Cache-Control", "private, no-store")
		v, ok := strings.CutPrefix(c.GetHeader("Authorization"), "Bearer ")
		if !ok || len(v) != 64 {
			c.AbortWithStatus(401)
			return
		}
		h := sha256.Sum256([]byte(v))
		var op string
		err := db.QueryRow(c.Request.Context(), "SELECT operator FROM editor_sessions WHERE hash=$1 AND expires_at>now()", h[:]).Scan(&op)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				c.AbortWithStatus(401)
			} else {
				c.AbortWithStatus(503)
			}
			return
		}
		c.Set("operator", op)
		c.Set("edit_hash", h[:])
		c.Next()
	})
	g.POST("/logout", func(c *gin.Context) {
		_, err := db.Exec(c.Request.Context(), "DELETE FROM editor_sessions WHERE hash=$1", c.MustGet("edit_hash"))
		if err != nil {
			c.Status(503)
			return
		}
		c.Status(204)
	})
	g.GET("/tasks", func(c *gin.Context) {
		queryJSON(c, db, `SELECT jsonb_build_object('tasks',coalesce(jsonb_agg(to_jsonb(t) ORDER BY created_at DESC),'[]'),'scopes',$1::jsonb)
 FROM (SELECT id,world,chapter,instruction,status,round,attempt,created_at FROM editor_tasks ORDER BY created_at DESC LIMIT 100)t`, editorScopes)
	})
	g.POST("/tasks", func(c *gin.Context) {
		var v struct {
			World, Chapter string
			Instruction    string `binding:"required,max=1400"`
			RequestID      string `json:"request_id" binding:"uuid,required"`
		}
		if !editorRequest(c, &v) {
			return
		}
		v.Instruction = strings.TrimSpace(v.Instruction)
		allowed := false
		for _, s := range editorScopes {
			if s.World == v.World && s.Chapter == v.Chapter {
				allowed = true
			}
		}
		if !allowed || v.Instruction == "" {
			c.Status(400)
			return
		}
		var id string
		err := pgx.BeginFunc(c.Request.Context(), db, func(tx pgx.Tx) error {
			err := tx.QueryRow(c.Request.Context(), `INSERT INTO editor_tasks(request_id,world,chapter,instruction,author,base,base_head)
 SELECT $1,$2,$3,$4,$5,base,(SELECT task FROM editor_heads WHERE world=$2 AND chapter=$3) FROM editor_snapshot($2,$3) base WHERE base IS NOT NULL
 ON CONFLICT(request_id) DO NOTHING RETURNING id`, v.RequestID, v.World, v.Chapter, v.Instruction, c.GetString("operator")).Scan(&id)
			if errors.Is(err, pgx.ErrNoRows) {
				return tx.QueryRow(c.Request.Context(), "SELECT id FROM editor_tasks WHERE request_id=$1 AND world=$2 AND chapter=$3 AND instruction=$4 AND author=$5", v.RequestID, v.World, v.Chapter, v.Instruction, c.GetString("operator")).Scan(&id)
			}
			if err == nil {
				return editorEvent(c.Request.Context(), tx, id, "created", "已保存编辑意见与当时公开的章节、场景", nil)
			}
			return err
		})
		if err != nil {
			c.JSON(409, gin.H{"error": "范围尚未发布或请求号冲突"})
			return
		}
		c.JSON(200, gin.H{"id": id})
	})
	g.GET("/tasks/:id", func(c *gin.Context) {
		if !uuid(c.Param("id")) {
			c.Status(400)
			return
		}
		queryJSON(c, db, `SELECT (to_jsonb(t)-ARRAY['lease','due','request_id','base'])||jsonb_build_object('base',base,'events',coalesce((SELECT jsonb_agg(to_jsonb(e)-'task' ORDER BY id) FROM editor_events e WHERE task=t.id),'[]'))
 FROM editor_tasks t WHERE id=$1`, c.Param("id"))
	})
	g.GET("/tasks/:id/preview", func(c *gin.Context) {
		if !uuid(c.Param("id")) {
			c.Status(400)
			return
		}
		queryJSON(c, db, `WITH t AS (SELECT * FROM editor_tasks WHERE id=$1 AND status IN ('review','published','conflict') AND round::text=$2 AND attempt::text=$3 AND jsonb_array_length(draft)>0),
        edits AS (SELECT coalesce(jsonb_object_agg(d->>'key',jsonb_build_object('original',o.txt,'text',d->'text')),'{}') patch
        FROM t CROSS JOIN LATERAL jsonb_array_elements(t.draft) d JOIN LATERAL (
          SELECT b->>'text' txt FROM jsonb_array_elements(CASE WHEN jsonb_typeof(t.base#>'{chapter,reader_passages}')='array' THEN t.base#>'{chapter,reader_passages}' ELSE t.base#>'{chapter,blocks}' END)b
          WHERE 'block/'||(b->>'key')=d->>'key'
          UNION ALL SELECT s.value#>>'{scene,text}' FROM jsonb_each(t.base->'scenes') s WHERE 'scene/'||s.key=d->>'key'
        ) o ON true)
        SELECT jsonb_build_object('chapter',editor_render(t.base->'chapter',patch),'scenes',coalesce((SELECT jsonb_object_agg(key,editor_render(value,patch)) FROM jsonb_each(t.base->'scenes')),'{}')) FROM t,edits`, c.Param("id"), c.Query("round"), c.Query("attempt"))
	})
	g.POST("/tasks/:id/:action", func(c *gin.Context) {
		var v struct {
			Round   int    `binding:"min=1"`
			Attempt int    `binding:"min=1"`
			Note    string `binding:"max=1400"`
		}
		if !uuid(c.Param("id")) {
			c.Status(400)
			return
		}
		if !editorRequest(c, &v) {
			return
		}
		action := c.Param("action")
		if action != "approve" && action != "return" {
			c.Status(404)
			return
		}
		if action == "return" && strings.TrimSpace(v.Note) == "" {
			c.Status(400)
			return
		}
		err := editorDecision(c.Request.Context(), db, c.Param("id"), action, c.GetString("operator"), v.Note, v.Round, v.Attempt)
		if err != nil {
			switch {
			case errors.Is(err, errEditStale):
				c.JSON(409, gin.H{"error": errEditStale.Error()})
			case errors.Is(err, pgx.ErrNoRows):
				c.Status(404)
			default:
				c.JSON(503, gin.H{"error": "发布服务暂不可用，请重试"})
			}
			return
		}
		c.Status(204)
	})
}

var errEditStale = errors.New("此稿已变化，请刷新并核对最新版本")

func editorDecision(ctx context.Context, db *pgxpool.Pool, id, action, operator, note string, round, attempt int) error {
	stale := false
	err := pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		var world, chapter, status string
		var r, a int
		var draft []editChange
		var base editSnapshot
		err := tx.QueryRow(ctx, "SELECT world,chapter,status,round,attempt,draft,base FROM editor_tasks WHERE id=$1 FOR UPDATE", id).Scan(&world, &chapter, &status, &r, &a, &draft, &base)
		if err != nil {
			return err
		}
		if r != round || a != attempt {
			return errEditStale
		}
		if action == "return" {
			if status != "review" && status != "failed" {
				return errEditStale
			}
			if err = editorEvent(ctx, tx, id, "returned", operator+" · "+note, draft); err != nil {
				return err
			}
			_, err = tx.Exec(ctx, `UPDATE editor_tasks SET round=round+1,attempt=1,status='evidence',feedback=$2,issue='',retries=0,due=now(),lease=NULL WHERE id=$1`, id, note)
			return err
		}
		if status == "published" {
			return nil
		}
		if status != "review" {
			return errEditStale
		}
		if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,33))", world+"/"+chapter); err != nil {
			return err
		}
		var same bool
		var patch map[string]copyField
		err = tx.QueryRow(ctx, `SELECT t.base=editor_snapshot(t.world,t.chapter) AND t.base_head IS NOT DISTINCT FROM h.task,coalesce(h.patch,'{}') FROM editor_tasks t LEFT JOIN editor_heads h USING(world,chapter) WHERE t.id=$1`, id).Scan(&same, &patch)
		if err != nil {
			return err
		}
		if !same {
			stale = true
			if _, err = tx.Exec(ctx, "UPDATE editor_tasks SET status='conflict',issue='公开定稿已更新，请按最新版本新建任务' WHERE id=$1", id); err != nil {
				return err
			}
			return editorEvent(ctx, tx, id, "conflict", "公开定稿已变化，未覆盖新版本", nil)
		}
		fields := base.fields()
		for _, d := range draft {
			before := fields[d.Key]
			original := before
			if old, ok := patch[d.Key]; ok && old.Text == before {
				original = old.Original
			}
			patch[d.Key] = copyField{original, d.Text}
		}
		if _, err = tx.Exec(ctx, `INSERT INTO editor_heads(world,chapter,task,patch) VALUES($1,$2,$3,$4) ON CONFLICT(world,chapter) DO UPDATE SET task=excluded.task,patch=excluded.patch`, world, chapter, id, patch); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, "UPDATE editor_tasks SET status='published',reviewer=$2,published_at=now() WHERE id=$1", id, operator); err != nil {
			return err
		}
		return editorEvent(ctx, tx, id, "published", operator+" 审核通过，整项发布", draft)
	})
	if err != nil {
		return err
	}
	if stale {
		return errEditStale
	}
	return nil
}
