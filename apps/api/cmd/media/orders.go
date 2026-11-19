package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"anime-agent/api/internal/aiprovider"
	"anime-agent/api/internal/billing"
	"anime-agent/api/internal/knowledge"
	"anime-agent/api/internal/modelusage"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func workOrders(args []string) error {
	flags := flag.NewFlagSet("process-orders", flag.ContinueOnError)
	once := flags.Bool("once", false, "process at most one matching order")
	if err := flags.Parse(args); err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	client, err := aiprovider.Config("MEDIA_AI")
	if err != nil {
		return err
	}
	if os.Getenv("MEDIA_AI_API_KEY") == "" {
		return errors.New("set MEDIA_AI_API_KEY for reader orders")
	}
	if client.Account == "" {
		client.Account = "default"
	}
	client.Purpose = "image"
	db, err := knowledge.Open(ctx)
	if err != nil {
		return err
	}
	defer db.Close()
	root, err := os.UserCacheDir()
	if err != nil {
		return err
	}
	client.Record = func(ctx context.Context, e modelusage.Event) error {
		return modelusage.Deliver(ctx, db, modelusage.RecoveryDir(), e)
	}
	if !*once {
		// Keep the checkout available while a long-running image is generated.
		go func() {
			ticker := time.NewTicker(15 * time.Second)
			defer ticker.Stop()
			for {
				_, e := db.Exec(ctx, `INSERT INTO billing_preview_workers VALUES ($1,$2,$3,now()+interval '45 seconds')
 ON CONFLICT(endpoint,provider_account,model) DO UPDATE SET available_until=excluded.available_until`, client.Base, client.Account, client.Model)
				if e != nil && ctx.Err() == nil {
					log.Printf("reader order availability: %v", e)
				}
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
				}
			}
		}()
	}
	for ctx.Err() == nil {
		var id string
		err = db.QueryRow(ctx, `SELECT o.id::text FROM billing_orders o JOIN billing_rates r ON r.id=o.rate_id
 WHERE o.automatic AND o.status='reserved' AND (r.endpoint,r.provider_account,r.model)=($1,$2,$3) ORDER BY o.seq LIMIT 1`, client.Base, client.Account, client.Model).Scan(&id)
		if err == nil {
			err = processOrder(ctx, db, id, client, root)
		}
		if errors.Is(err, pgx.ErrNoRows) {
			err = nil
		}
		if *once {
			return err
		}
		if err != nil && ctx.Err() == nil {
			log.Printf("reader order %s: %v", id, err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(3 * time.Second):
		}
	}
	return nil
}

func processOrder(ctx context.Context, db *pgxpool.Pool, order string, client aiprovider.Client, root string) error {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Minute)
	defer cancel()
	id, req, err := billing.ReadRequest(ctx, db, order)
	if err != nil {
		return err
	}
	client.Size = req.Size
	return fulfill(ctx, db, id, client, brief(req.Brief), req.Size, root)
}
