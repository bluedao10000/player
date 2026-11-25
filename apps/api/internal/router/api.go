package router

import (
	"context"
	"crypto/subtle"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"anime-agent/api/internal/handler/billing"
	"anime-agent/api/internal/handler/knowledge"
	"anime-agent/api/internal/handler/reader"
	"anime-agent/api/internal/handler/workbench"
)

func New(db *pgxpool.Pool, logger *zap.Logger) *gin.Engine {
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		start := time.Now()
		c.Next()
		logger.Info("request", zap.String("method", c.Request.Method), zap.String("route", c.FullPath()), zap.Int("status", c.Writer.Status()), zap.Duration("duration", time.Since(start)))
	}, gin.Recovery())
	engine.GET("/healthz", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
		defer cancel()
		if db.Ping(ctx) != nil {
			c.JSON(503, gin.H{"service": "api", "status": "unavailable"})
			return
		}
		c.JSON(200, gin.H{"service": "api", "status": "ok"})
	})
	Register(engine, db)
	return engine
}

// Register installs public, authenticated reader, and operator routes.
func Register(r *gin.Engine, db *pgxpool.Pool) {
	admin := adminGroup(r)
	knowledge.AdminRoutes(admin, db)
	billing.AdminRoutes(admin, db)
	reader.AdminRoutes(admin, db)
	billing.PaymentRoutes(r, db)
	reader.Routes(r, db)
	knowledge.Routes(r, db)
	workbench.Routes(r, db, operatorFromBearer)
	if path := os.Getenv("WORKBENCH_CONTRACT_PATH"); path != "" {
		r.Match([]string{"GET", "HEAD"}, "/docs/api-contract.md", func(c *gin.Context) {
			c.Header("Content-Type", "text/plain; charset=utf-8")
			c.Header("X-Content-Type-Options", "nosniff")
			c.File(path)
		})
	}
}

func adminGroup(r *gin.Engine) *gin.RouterGroup {
	return r.Group("/api/v1/admin/knowledge", func(c *gin.Context) {
		operator := operatorFromBearer(c.GetHeader("Authorization"))
		if operator == "" {
			c.AbortWithStatus(401)
			return
		}
		c.Set("operator", operator)
		c.Header("Cache-Control", "no-store")
		c.Next()
	})
}

func operatorFromBearer(header string) string {
	token, operator := os.Getenv("KNOWLEDGE_ADMIN_TOKEN"), os.Getenv("KNOWLEDGE_ADMIN_OPERATOR")
	if operator == "" || len(token) < 32 || subtle.ConstantTimeCompare([]byte(header), []byte("Bearer "+token)) != 1 {
		return ""
	}
	return operator
}
