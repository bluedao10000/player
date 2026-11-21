package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/user"
	"strconv"

	catalog "anime-agent/api/internal/model/knowledge"
	"anime-agent/api/internal/repository/database"
	"anime-agent/api/internal/service/knowledge"
	"anime-agent/api/internal/service/modelusage"
	queries "anime-agent/api/internal/repository/queries/knowledge"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: knowledge quality|review|rebuild|usage-import")
	}
	ctx := context.Background()
	db, err := database.Open(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	switch os.Args[1] {
	case "episode-register":
		err = knowledge.RegisterProductionPlan(ctx, db, os.Stdin)
	case "episode-capture-claim":
		err = knowledge.ClaimCapture(ctx, db, os.Stdout)
	case "episode-plan-load":
		err = knowledge.LoadEpisodePlans(ctx, db)
	case "episode-evidence-due":
		rows, e := db.Query(ctx, queries.ListEvidenceDue)
		if e != nil {
			log.Fatal(e)
		}
		for rows.Next() {
			var world string
			var episode int
			if e = rows.Scan(&world, &episode); e != nil {
				log.Fatal(e)
			}
			fmt.Println(world, episode)
		}
		err = rows.Err()
		rows.Close()
	case "episode-evidence-import":
		err = knowledge.ImportEpisodeCapture(ctx, db, os.Stdin)
	case "episode-inventory-import":
		var snapshot struct {
			Checked string
			Works   json.RawMessage
		}
		if err = json.NewDecoder(os.Stdin).Decode(&snapshot); err == nil {
			_, err = db.Exec(ctx, queries.ImportInventory, snapshot.Checked, snapshot.Works)
		}
	case "usage-import":
		f := flag.NewFlagSet("usage-import", flag.ExitOnError)
		dir := f.String("dir", "", "model usage outbox directory")
		f.Parse(os.Args[2:])
		if *dir == "" {
			log.Fatal("--dir required")
		}
		err = modelusage.Replay(ctx, db, *dir)
	case "quality", "rebuild", "diff", "project":
		f := flag.NewFlagSet(os.Args[1], flag.ExitOnError)
		source := f.String("source", "all", "source name")
		work := f.String("work-id", "", "canonical UUID")
		version := f.String("version", "", "archived version (default current)")
		old := f.String("from", "", "prior version")
		next := f.String("to", "", "next version")
		key := f.String("record", "", "source record key")
		f.Parse(os.Args[2:])
		if os.Args[1] == "diff" {
			v, e := knowledge.DiffSnapshots(ctx, db, *source, *old, *next, *key)
			if e != nil {
				log.Fatal(e)
			}
			if e = json.NewEncoder(os.Stdout).Encode(v); e != nil {
				log.Fatal(e)
			}
			return
		}
		if os.Args[1] == "project" {
			if e := knowledge.Reproject(ctx, db, *source); e != nil {
				log.Fatal(e)
			}
			return
		}
		if os.Args[1] == "quality" {
			r, e := knowledge.Quality(ctx, db, *source, *work)
			err = e
			if e == nil {
				fmt.Println(string(r))
			}
		} else {
			err = knowledge.Rebuild(ctx, db, *source, *version)
		}
	case "review":
		if len(os.Args) < 3 {
			log.Fatal("review list | approve ID REASON")
		}
		switch os.Args[2] {
		case "list":
			var v string
			err = db.QueryRow(ctx, queries.ListReviewCandidates).Scan(&v)
			if err == nil {
				fmt.Println(v)
			}
		case "approve":
			if len(os.Args) != 5 {
				log.Fatal("review approve ID 'reason'")
			}
			id, e := strconv.ParseInt(os.Args[3], 10, 64)
			if e != nil {
				log.Fatal(e)
			}
			u, e := user.Current()
			if e != nil {
				log.Fatal(e)
			}
			err = knowledge.Decide(ctx, db, catalog.Decision{Action: "approve", Candidate: id, Reason: os.Args[4]}, u.Username, "local-cli")
		default:
			log.Fatal("unknown review command")
		}
	default:
		log.Fatal("unknown command")
	}
	if err != nil {
		log.Fatal(err)
	}
}
