package scheduler

import (
	"context"
	"errors"
	"log"
	"os"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	queries "anime-agent/api/internal/repository/queries/knowledge"
	"anime-agent/api/internal/service/knowledge"
	"anime-agent/api/internal/service/modelusage"
)

// Split the episode scheduler by the location of its model gateway and artwork.
// Existing single-node deployments retain their original scheduler.
func RunLane(ctx context.Context, db *pgxpool.Pool, lane, agent string) error {
	if agent == "" || (lane != "research" && lane != "media") {
		return errors.New("episode lane requires a local Pi URL and research or media role")
	}
	node, err := requireNodeName()
	if err != nil {
		return err
	}
	role := lane + "-lane"
	var jobs sync.WaitGroup
	defer jobs.Wait()
	startAgentStatusLoop(ctx, db, node, agent, &jobs)
	if lane == "research" {
		startDailySnapshotLoop(ctx, db, node, role, &jobs)
	}
	go func() {
		if err := serveBudget(ctx, db); err != nil && ctx.Err() == nil {
			log.Printf("episode budget server: %v", err)
		}
	}()
	maintenanceReporter := newLoopReporter(db, node, role, "maintenance", time.Minute, false)
	workLoop := "research"
	if lane == "media" {
		workLoop = "media"
	}
	workReporter := newLoopReporter(db, node, role, workLoop, 5*time.Second, false)
	usageReporter := newLoopReporter(db, node, role, "usage", 5*time.Second, false)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	var nextMaintenance time.Time
	for ctx.Err() == nil {
		if time.Now().After(nextMaintenance) {
			var cycleErr error
			if lane == "research" {
				_, err := db.Exec(ctx, queries.CompleteReleasedPlans)
				addCycleError(ctx, &cycleErr, "observe reader release", err)
				_, err = db.Exec(ctx, queries.InspectPlanInventory)
				addCycleError(ctx, &cycleErr, "inspect production scope", err)
				addCycleError(ctx, &cycleErr, "admit approved gaps", knowledge.AdmitEpisode(ctx, db))
			} else {
				addCycleError(ctx, &cycleErr, "acceptance", knowledge.AcceptPlannedEpisodes(ctx, db))
			}
			maintenanceReporter.record(ctx, cycleErr, nil)
			nextMaintenance = time.Now().Add(time.Minute)
		}
		var workErr error
		if lane == "research" {
			_, err := db.Exec(ctx, queries.ResumeBudgetWait)
			addCycleError(ctx, &workErr, "resume budget wait", err)
			addCycleError(ctx, &workErr, "draft episode plan", draftPlan(ctx, db, agent))
			addCycleError(ctx, &workErr, "episode configuration", prepareConfiguration(ctx, db, agent))
			_, err = db.Exec(ctx, queries.Configuration["ObserveLocalAcceptance"])
			addCycleError(ctx, &workErr, "observe local acceptance", err)
			addCycleError(ctx, &workErr, "research", knowledge.ResearchIfDue(ctx, db, false))
		} else {
			addCycleError(ctx, &workErr, "media", knowledge.SyncMedia(ctx, db, agent))
		}
		workReporter.record(ctx, workErr, nil)
		reportLoop(ctx, usageReporter, "usage", syncLaneUsage(ctx, db, lane, agent))
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
	return nil
}

func syncLaneUsage(ctx context.Context, db *pgxpool.Pool, lane, agent string) error {
	pullErr := modelusage.Pull(ctx, db, agent)
	// The two lanes share an archive. Only an opted-in research lane consumes its files.
	if lane == "research" && os.Getenv(modelusage.LegacyOutboxEnv) != "" {
		return errors.Join(pullErr, modelusage.ReplayConfigured(ctx, db))
	}
	return pullErr
}
