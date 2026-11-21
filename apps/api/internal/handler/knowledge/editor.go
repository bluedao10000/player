package knowledge

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"

	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	catalog "anime-agent/api/internal/model/knowledge"
	queries "anime-agent/api/internal/repository/queries/knowledge"
	"anime-agent/api/internal/repository/workbench"
	domain "anime-agent/api/internal/service/knowledge"
)

var editorScopes = []catalog.EditorScope{{World: "naruto-tv-2002", Chapter: "enter-naruto", Name: "火影忍者 · 第 1 集"}, {World: "tidepost", Chapter: "night-courier", Name: "潮汐邮局 · 海风里的夜班邮差"}}

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
		_, err := db.Exec(c.Request.Context(), queries.CreateEditorSession, hash[:], c.GetString("operator"))
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
		err := db.QueryRow(c.Request.Context(), queries.EditorSessionOperator, h[:]).Scan(&op)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				c.AbortWithStatus(401)
			} else {
				c.AbortWithStatus(503)
			}
			return
		}
		email, mailLogin := strings.CutPrefix(op, "email:")
		editor := !mailLogin
		if mailLogin {
			if err := db.QueryRow(c.Request.Context(), queries.WorkbenchEditorGranted, email).Scan(&editor); err != nil {
				c.AbortWithStatus(503)
				return
			}

		}
		c.Set("superadmin", !mailLogin)
		c.Set("email", email)
		c.Set("editor", editor)
		if !editor && c.FullPath() != "/api/v1/editor/account" && c.FullPath() != "/api/v1/editor/logout" {
			c.AbortWithStatusJSON(403, gin.H{"error": "当前账户尚未开通编辑权限"})
			return
		}
		c.Set("operator", op)
		c.Set("edit_hash", h[:])
		c.Next()
	})
	g.GET("/account", func(c *gin.Context) {
		email := c.GetString("email")
		if !strings.Contains(email, "@") {
			email = ""
		}
		c.JSON(200, gin.H{"email": email, "editor": c.GetBool("editor"), "superadmin": c.GetBool("superadmin")})
	})
	g.POST("/editor-access", func(c *gin.Context) {
		if !c.GetBool("superadmin") {
			c.AbortWithStatusJSON(403, gin.H{"error": "仅超级管理员可管理编辑权限"})
			return
		}
		var v struct {
			Email  string `binding:"required,email,max=254"`
			Action string `binding:"required,oneof=grant revoke"`
		}
		if !editorRequest(c, &v) {
			return
		}
		query := queries.GrantWorkbenchEditor
		if v.Action == "revoke" {
			query = queries.RevokeWorkbenchEditor
		}
		if _, err := db.Exec(c.Request.Context(), query, strings.ToLower(strings.TrimSpace(v.Email))); err != nil {
			c.Status(503)
			return
		}
		c.Status(204)
	})
	g.GET("/plans", func(c *gin.Context) {
		var data []byte
		err := db.QueryRow(c.Request.Context(), queries.PlanDesk).Scan(&data)
		writeJSON(c, data, err)
	})
	g.POST("/plans", func(c *gin.Context) {
		var v struct {
			ID          string `binding:"required,uuid"`
			World       string `binding:"required,max=128"`
			First       int    `binding:"min=1,max=100000"`
			Last        int    `binding:"gtefield=First,max=100000"`
			Budget      int64  `binding:"min=1"`
			Instruction string `binding:"required,max=1400"`
		}
		if !editorRequest(c, &v) {
			return
		}
		if v.Last-v.First >= 1000 {
			c.Status(400)
			return
		}
		_, err := db.Exec(c.Request.Context(), queries.SubmitPlan, v.ID, v.World, v.First, v.Last, v.Budget, v.Instruction, c.GetString("operator"))
		if err != nil {
			c.Status(409)
			return
		}
		c.Status(204)
	})
	g.POST("/plans/:id/messages", func(c *gin.Context) {
		var v struct {
			Revision    int    `binding:"min=1"`
			Instruction string `binding:"required,max=1400"`
		}
		if !editorRequest(c, &v) {
			return
		}
		if !catalog.ValidUUID(c.Param("id")) {
			c.Status(400)
			return
		}
		tag, err := db.Exec(c.Request.Context(), queries.RevisePlan, c.Param("id"), v.Revision, v.Instruction, c.GetString("operator"))
		if err != nil || tag.RowsAffected() != 1 {
			c.Status(409)
			return
		}
		c.Status(204)
	})
	g.POST("/logout", func(c *gin.Context) {
		_, err := db.Exec(c.Request.Context(), queries.DeleteEditorSession, c.MustGet("edit_hash"))
		if err != nil {
			c.Status(503)
			return
		}
		c.Status(204)
	})
	g.GET("/tasks", func(c *gin.Context) {
		ctx, cancel := readContext(c)
		defer cancel()
		body, err := workbench.Tasks(ctx, db, editorScopes)
		writeJSON(c, body, err)
	})
	g.POST("/tasks", func(c *gin.Context) {
		var v struct {
			World, Chapter, Kind string
			Instruction          string `binding:"required,max=1400"`
			RequestID            string `json:"request_id" binding:"uuid,required"`
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
		if v.Kind == "" {
			v.Kind = "chapter"
		}
		if !allowed || v.Instruction == "" || (v.Kind != "chapter" && v.Kind != "profiles") {
			c.Status(400)
			return
		}
		id, err := domain.CreateEdit(c.Request.Context(), db, v.RequestID, v.World, v.Chapter, v.Kind, v.Instruction, c.GetString("operator"))
		if err != nil {
			c.JSON(409, gin.H{"error": "范围尚未发布或请求号冲突"})
			return
		}
		c.JSON(200, gin.H{"id": id})
	})
	g.GET("/tasks/:id", func(c *gin.Context) {
		if !catalog.ValidUUID(c.Param("id")) {
			c.Status(400)
			return
		}
		ctx, cancel := readContext(c)
		defer cancel()
		body, err := workbench.Task(ctx, db, c.Param("id"))
		writeJSON(c, body, err)
	})
	g.GET("/tasks/:id/preview", func(c *gin.Context) {
		if !catalog.ValidUUID(c.Param("id")) {
			c.Status(400)
			return
		}
		ctx, cancel := readContext(c)
		defer cancel()
		body, err := workbench.Preview(ctx, db, c.Param("id"), c.Query("round"), c.Query("attempt"))
		writeJSON(c, body, err)
	})
	g.POST("/tasks/:id/:action", func(c *gin.Context) {
		var v struct {
			Round   int    `binding:"min=1"`
			Attempt int    `binding:"min=1"`
			Note    string `binding:"max=1400"`
		}
		if !catalog.ValidUUID(c.Param("id")) {
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
		err := domain.ReviewEdit(c.Request.Context(), db, c.Param("id"), action, c.GetString("operator"), v.Note, v.Round, v.Attempt)
		if err != nil {
			switch {
			case errors.Is(err, domain.ErrEditStale):
				c.JSON(409, gin.H{"error": domain.ErrEditStale.Error()})
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
