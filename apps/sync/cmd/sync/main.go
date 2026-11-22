package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"anime-agent/api/internal/config"
	"anime-agent/api/internal/observability"
	"anime-agent/api/internal/repository/database"
	"anime-agent/api/internal/service/knowledge"
	"anime-agent/api/sync/internal/service/scheduler"
)

func main() {
	accept := flag.Bool("accept-scope", false, "validate and publish the selected episode scope once")
	research := flag.Bool("research", false, "process one due episode research topic and exit")
	once := flag.Bool("once", false, "probe enabled sources once and exit")
	source := flag.String("source", "", "optional source name")
	refresh := flag.Bool("refresh", false, "with -once: revalidate and reconcile the current snapshot")
	resumeImages := flag.String("resume-image-work", "", "requeue failed image provider requests for one work after upstream recovery")
	lane := flag.String("episode-lane", "", "run the research or media episode scheduler against its local Pi")
	flag.Parse()
	_, closeLog, err := observability.NewLogger("sync")
	if err != nil {
		log.Fatal(err)
	}
	defer closeLog()
	cfg := config.LoadSync()
	if *refresh && !*once {
		log.Fatal("-refresh requires -once")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	db, err := database.Open(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if *lane != "" {
		if err := scheduler.RunLane(ctx, db, *lane, cfg.AgentURL); err != nil {
			log.Fatal(err)
		}
		return
	}
	if *resumeImages != "" {
		count, err := knowledge.ResumeImageProviderFailures(ctx, db, *resumeImages)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("requeued %d image tasks for %s", count, *resumeImages)
		return
	}
	if *accept {
		if err := knowledge.AcceptEpisodeScope(ctx, db); err != nil {
			log.Fatal(err)
		}
		if err := knowledge.AcceptThirdEpisode(ctx, db); err != nil {
			log.Fatal(err)
		}
		return
	}
	if err := scheduler.Run(ctx, db, cfg, scheduler.Options{Once: *once, Research: *research, Refresh: *refresh, Source: *source}); err != nil {
		log.Fatal(err)
	}
}
