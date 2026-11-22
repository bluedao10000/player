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
	var node string
	if !options.Once && !options.Research {
		var err error
		node, err = requireNodeName()
		if err != nil {
			return err
		}
		startDailySnapshotLoop(ctx, db, node, "sync", &jobs)
		if cfg.AgentURL != "" {
			startAgentStatusLoop(ctx, db, node, cfg.AgentURL, &jobs)
		}
	}
	if !options.Once && !options.Research {
		base := cfg.AgentURL
		if base != "" {
			reporter := newLoopReporter(db, node, "sync", "editorial", 2*time.Second, false)
			jobs.Go(func() {
				for ctx.Err() == nil {
					reportLoop(ctx, reporter, "editorial", knowledge.ProcessEditorial(ctx, db, base))
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
				reporter := newLoopReporter(db, node, "sync", "episode-planner", 15*time.Second, false)
				ticker := time.NewTicker(15 * time.Second)
				defer ticker.Stop()
				for ctx.Err() == nil {
					var cycleErr error
					_, err := db.Exec(ctx, queries.InspectPlanInventory)
					addCycleError(ctx, &cycleErr, "episode inventory", err)
					addCycleError(ctx, &cycleErr, "episode planner", draftPlan(ctx, db, base))
					addCycleError(ctx, &cycleErr, "episode configuration", prepareConfiguration(ctx, db, base))
					_, err = db.Exec(ctx, queries.ResumeBudgetWait)
					addCycleError(ctx, &cycleErr, "episode budget retry", err)
					_, err = db.Exec(ctx, queries.Configuration["ObserveLocalAcceptance"])
					addCycleError(ctx, &cycleErr, "episode local acceptance", err)
					reporter.record(ctx, cycleErr, nil)
					select {
					case <-ctx.Done():
						return
					case <-ticker.C:
					}
				}
			})
		}
		jobs.Go(func() {
			reporter := newLoopReporter(db, node, "sync", "usage", 5*time.Second, false)
			for ctx.Err() == nil {
				var cycleErr error
				addCycleError(ctx, &cycleErr, "model receipt recovery", modelusage.Replay(ctx, db, modelusage.RecoveryDir()))
				if base != "" {
					addCycleError(ctx, &cycleErr, "model usage sync", modelusage.Pull(ctx, db, base))
				}
				reporter.record(ctx, cycleErr, nil)
				select {
				case <-ctx.Done():
					return
				case <-time.After(5 * time.Second):
				}
			}
		})

		jobs.Go(func() {
			reporter := newLoopReporter(db, node, "sync", "episode-plans", 45*time.Second, false)
			for ctx.Err() == nil {
				reportLoop(ctx, reporter, "episode plans", knowledge.LoadEpisodePlans(ctx, db))
				select {
				case <-ctx.Done():
					return
				case <-time.After(45 * time.Second):
				}
			}
		})
		jobs.Go(func() {
			reporter := newLoopReporter(db, node, "sync", "research", 8*time.Second, false)
			for ctx.Err() == nil {
				reportLoop(ctx, reporter, "episode research", knowledge.ResearchIfDue(ctx, db, false))
				select {
				case <-ctx.Done():
					return
				case <-time.After(8 * time.Second):
				}
			}
		})
		jobs.Go(func() {
			reporter := newLoopReporter(db, node, "sync", "acceptance", time.Minute, false)
			ticker := time.NewTicker(time.Minute)
			defer ticker.Stop()
			for {
				var cycleErr error
				addCycleError(ctx, &cycleErr, "configured acceptance", knowledge.AcceptPlannedEpisodes(ctx, db))
				addCycleError(ctx, &cycleErr, "episode acceptance", knowledge.AcceptEpisodeScope(ctx, db))
				addCycleError(ctx, &cycleErr, "incremental acceptance", knowledge.AcceptThirdEpisode(ctx, db))
				reporter.record(ctx, cycleErr, nil)
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
			reporter := newLoopReporter(db, node, "sync", "media", 4*time.Second, false)
			ticker := time.NewTicker(4 * time.Second)
			defer ticker.Stop()
			for {
				reportLoop(ctx, reporter, "media", knowledge.SyncMedia(ctx, db, cfg.AgentURL))
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
	sourceReporter := newLoopReporter(db, node, "sync", "source-sync", time.Minute, false)
	qualityReporter := newLoopReporter(db, node, "sync", "quality-scan", time.Minute, false)
	for {
		err := knowledge.Sweep(ctx, db, options.Source, options.Once, options.Refresh)
		reportLoop(ctx, sourceReporter, "source sync", err)
		if err != nil && ctx.Err() == nil {
			if options.Once {
				return err
			}
		}
		if options.Once || ctx.Err() != nil {
			return nil
		}

		reportLoop(ctx, qualityReporter, "scheduled quality", knowledge.QualityIfDue(ctx, db))
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(time.Minute):
		}
	}
}
