package scheduler

import (
	"context"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	queries "anime-agent/api/internal/repository/queries/knowledge"
	"anime-agent/api/internal/service/knowledge"
	"anime-agent/api/internal/service/modelusage"
)

// Split the episode scheduler by the location of its model gateway and artwork.
// Existing single-node deployments retain their original scheduler.
func RunLane(ctx context.Context, db *pgxpool.Pool, lane, agent string) {
	if agent == "" || (lane != "research" && lane != "media") {
		log.Fatal("episode lane requires a local Pi URL and research or media role")
	}
	go func() {
		if err := serveBudget(ctx, db); err != nil && ctx.Err() == nil {
			log.Fatal(err)
		}
	}()
	run := func(name string, fn func(context.Context) error) {
		if err := fn(ctx); err != nil && ctx.Err() == nil {
			log.Printf("%s: %v", name, err)
		}
	}
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	var nextMaintenance time.Time
	for ctx.Err() == nil {
		if time.Now().After(nextMaintenance) {
			if lane == "research" {
				run("observe reader release", func(ctx context.Context) error { _, err := db.Exec(ctx, queries.CompleteReleasedPlans); return err })
				run("inspect production scope", func(ctx context.Context) error { _, err := db.Exec(ctx, queries.InspectPlanInventory); return err })
				run("admit approved gaps", func(ctx context.Context) error { return knowledge.AdmitEpisode(ctx, db) })
			} else {
				run("acceptance", func(ctx context.Context) error { return knowledge.AcceptPlannedEpisodes(ctx, db) })
			}
			nextMaintenance = time.Now().Add(time.Minute)
		}
		if lane == "research" {
			run("resume budget wait", func(ctx context.Context) error { _, err := db.Exec(ctx, queries.ResumeBudgetWait); return err })
			run("draft episode plan", func(ctx context.Context) error { return draftPlan(ctx, db, agent) })
			run("research", func(ctx context.Context) error { return knowledge.ResearchIfDue(ctx, db, false) })
		} else {
			run("media", func(ctx context.Context) error { return knowledge.SyncMedia(ctx, db, agent) })
		}
		run("usage", func(ctx context.Context) error { return modelusage.Pull(ctx, db, agent) })
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
