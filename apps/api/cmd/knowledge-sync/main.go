package main

import (
	"anime-agent/api/internal/knowledge"
	"anime-agent/api/internal/modelusage"
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	accept := flag.Bool("accept-scope", false, "validate and publish the selected episode scope once")
	research := flag.Bool("research", false, "process one due episode research topic and exit")
	once := flag.Bool("once", false, "probe enabled sources once and exit")
	source := flag.String("source", "", "optional source name")
	refresh := flag.Bool("refresh", false, "with -once: revalidate and reconcile the current snapshot")
	flag.Parse()
	if *refresh && !*once {
		log.Fatal("-refresh requires -once")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	db, err := knowledge.Open(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if *accept {
		if err := knowledge.AcceptEpisodeScope(ctx, db); err != nil {
			log.Fatal(err)
		}
		if err := knowledge.AcceptThirdEpisode(ctx, db); err != nil {
			log.Fatal(err)
		}
		return
	}
	if !*once && !*research {
		base := os.Getenv("KNOWLEDGE_RESEARCH_URL")
		go func() {
			for ctx.Err() == nil {
				if e := modelusage.Replay(ctx, db, modelusage.RecoveryDir()); e != nil && ctx.Err() == nil {
					log.Printf("model receipt recovery: %v", e)
				}
				if base != "" {
					if e := modelusage.Pull(ctx, db, base); e != nil && ctx.Err() == nil {
						log.Printf("model usage sync: %v", e)
					}
				}
				select {
				case <-ctx.Done():
					return
				case <-time.After(5 * time.Second):
				}
			}
		}()

		go func() {
			for ctx.Err() == nil {
				if e := knowledge.LoadEpisodePlans(ctx, db); e != nil {
					log.Printf("episode plans: %v", e)
				}
				select {
				case <-ctx.Done():
					return
				case <-time.After(45 * time.Second):
				}
			}
		}()
		go func() {
			for ctx.Err() == nil {
				if e := knowledge.ResearchIfDue(ctx, db, false); e != nil {
					log.Printf("episode research: %v", e)
				}
				select {
				case <-ctx.Done():
					return
				case <-time.After(8 * time.Second):
				}
			}
		}()
		go func() {
			ticker := time.NewTicker(time.Minute)
			defer ticker.Stop()
			for {
				if e := knowledge.AcceptPlannedEpisodes(ctx, db); e != nil && ctx.Err() == nil {
					log.Printf("configured acceptance: %v", e)
				}
				if err := knowledge.AcceptEpisodeScope(ctx, db); err != nil && ctx.Err() == nil {
					log.Printf("episode acceptance: %v", err)
				}
				if err := knowledge.AcceptThirdEpisode(ctx, db); err != nil && ctx.Err() == nil {
					log.Printf("incremental acceptance: %v", err)
				}
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
				}
			}
		}()
	}
	if !*once && !*research && os.Getenv("MEDIA_WORKER_ENABLED") == "true" {
		go func() {
			ticker := time.NewTicker(4 * time.Second)
			defer ticker.Stop()
			for {
				if e := knowledge.SyncMedia(ctx, db, os.Getenv("KNOWLEDGE_RESEARCH_URL")); e != nil && ctx.Err() == nil {
					log.Printf("media: %v", e)
				}
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
				}
			}
		}()
	}
	if *research {
		if os.Getenv("KNOWLEDGE_RESEARCH_URL") == "" {
			log.Fatal("KNOWLEDGE_RESEARCH_URL is required")
		}
		if err := knowledge.ResearchIfDue(ctx, db, true); err != nil {
			log.Fatal(err)
		}
		return
	}
	for {
		err = knowledge.Sweep(ctx, db, *source, *once, *refresh)
		if err != nil && ctx.Err() == nil {
			if *once {
				log.Fatal(err)
			}
			log.Print(err)
		}
		if *once || ctx.Err() != nil {
			return
		}

		if err = knowledge.QualityIfDue(ctx, db); err != nil && ctx.Err() == nil {
			log.Printf("scheduled quality: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Minute):
		}
	}
}
