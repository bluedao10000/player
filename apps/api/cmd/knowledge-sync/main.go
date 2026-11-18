package main

import (
	"anime-agent/api/internal/knowledge"
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
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
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Minute):
		}
	}
}
