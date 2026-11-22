package scheduler

import (
	"context"
	"errors"
	"log"
	"os"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"anime-agent/api/internal/config"
	queries "anime-agent/api/internal/repository/queries/knowledge"
	"anime-agent/api/internal/service/knowledge"
	"anime-agent/api/internal/service/modelusage"
)

type Options struct {
	Once, Research, Refresh bool
	Source                  string
}

func Run(ctx context.Context, db *pgxpool.Pool, cfg config.Sync, options Options) error {
	var jobs sync.WaitGroup
	defer jobs.Wait()
	if !options.Once && !options.Research {
		base := cfg.AgentURL
		if base != "" {
			jobs.Go(func() {
				for ctx.Err() == nil {
					if err := knowledge.ProcessEditorial(ctx, db, base); err != nil && ctx.Err() == nil {
						log.Printf("editorial: %v", err)
					}
					select {
					case <-ctx.Done():
						return
					case <-time.After(2 * time.Second):
					}
				}
			})
		}
		if base != "" && os.Getenv("EPISODE_BUDGET_BIND") != "" {
			jobs.Go(func() {
				if err := serveBudget(ctx, db); err != nil && ctx.Err() == nil {
					log.Printf("episode budget server: %v", err)
				}
			})
			jobs.Go(func() {
				ticker := time.NewTicker(15 * time.Second)
				defer ticker.Stop()
				for ctx.Err() == nil {
					if _, err := db.Exec(ctx, queries.InspectPlanInventory); err != nil && ctx.Err() == nil {
						log.Printf("episode inventory: %v", err)
					}
					if err := draftPlan(ctx, db, base); err != nil && ctx.Err() == nil {
						log.Printf("episode planner: %v", err)
					}
					select {
					case <-ctx.Done():
						return
					case <-ticker.C:
					}
				}
			})
		}
		jobs.Go(func() {
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
		})

		jobs.Go(func() {
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
		})
		jobs.Go(func() {
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
		})
		jobs.Go(func() {
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
		})
	}
	if !options.Once && !options.Research && cfg.MediaEnabled {
		jobs.Go(func() {
			ticker := time.NewTicker(4 * time.Second)
			defer ticker.Stop()
			for {
				if e := knowledge.SyncMedia(ctx, db, cfg.AgentURL); e != nil && ctx.Err() == nil {
					log.Printf("media: %v", e)
				}
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
				}
			}
		})
	}
	if options.Research {
		if cfg.AgentURL == "" {
			return errors.New("KNOWLEDGE_RESEARCH_URL is required")
		}
		return knowledge.ResearchIfDue(ctx, db, true)
	}
	for {
		err := knowledge.Sweep(ctx, db, options.Source, options.Once, options.Refresh)
		if err != nil && ctx.Err() == nil {
			if options.Once {
				return err
			}
			log.Print(err)
		}
		if options.Once || ctx.Err() != nil {
			return nil
		}

		if err = knowledge.QualityIfDue(ctx, db); err != nil && ctx.Err() == nil {
			log.Printf("scheduled quality: %v", err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(time.Minute):
		}
	}
}
