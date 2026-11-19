package main

import (
	"anime-agent/api/internal/knowledge"
	"anime-agent/api/internal/modelusage"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/user"
	"strconv"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: knowledge quality|review|rebuild|usage-import")
	}
	ctx := context.Background()
	db, err := knowledge.Open(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	switch os.Args[1] {
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
			err = db.QueryRow(ctx, "SELECT coalesce(jsonb_agg(to_jsonb(x)),'[]')::text FROM (SELECT * FROM knowledge_match_candidates ORDER BY id LIMIT 100)x").Scan(&v)
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
			err = knowledge.Decide(ctx, db, knowledge.Decision{Action: "approve", Candidate: id, Reason: os.Args[4]}, u.Username, "local-cli")
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
