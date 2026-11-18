package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"anime-agent/api/internal/knowledge"
	"github.com/gin-gonic/gin"
)

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
func run() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := knowledge.Open(ctx)
	if err != nil {
		return err
	}
	defer db.Close()
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())
	router.GET("/healthz", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
		defer cancel()
		if db.Ping(ctx) != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"service": "api", "status": "unavailable"})
			return
		}
		c.JSON(200, gin.H{"service": "api", "status": "ok"})
	})
	knowledge.Routes(router, db)
	return router.Run(net.JoinHostPort(envOr("API_HOST", "127.0.0.1"), envOr("API_PORT", "4311")))
}
func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
