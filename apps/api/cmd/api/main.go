package main

import (
	"context"
	"log"
	"net"
	"time"

	"anime-agent/api/internal/config"
	"anime-agent/api/internal/observability"
	"anime-agent/api/internal/repository/database"
	"anime-agent/api/internal/router"
)

func run() error {
	logger, closeLog, err := observability.NewLogger("api")
	if err != nil {
		return err
	}
	defer closeLog()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := database.Open(ctx)
	if err != nil {
		return err
	}
	defer db.Close()
	cfg := config.LoadAPI()
	return router.New(db, logger).Run(net.JoinHostPort(cfg.Host, cfg.Port))
}
func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
