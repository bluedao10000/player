package main

import (
	"database/sql"
	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func run() error {
	path := envOr("APP_DB_PATH", "../../data/app.sqlite")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		return err
	}

	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())
	router.GET("/healthz", func(c *gin.Context) {
		if err := db.PingContext(c.Request.Context()); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"service": "api", "status": "unavailable"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"service": "api", "status": "ok"})
	})
	return router.Run("127.0.0.1:" + envOr("API_PORT", "4311"))
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
