package scheduler

import (
	q "anime-agent/api/internal/repository/queries/knowledge"
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http"
	"os"
	"time"
)

func serveBudget(ctx context.Context, db *pgxpool.Pool) error {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /reserve", func(w http.ResponseWriter, r *http.Request) {
		var call struct {
			Subject, Call, Step string
			Allowance           int64
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048)).Decode(&call) != nil || call.Allowance < 1 {
			w.WriteHeader(400)
			return
		}
		var owner string
		if err := db.QueryRow(r.Context(), q.ReserveSubject, call.Subject, call.Call, call.Step, call.Allowance).Scan(&owner); err != nil {
			w.WriteHeader(503)
			return
		}
		if owner == "paused" {
			w.WriteHeader(429)
			return
		}
		json.NewEncoder(w).Encode(owner)
	})
	mux.HandleFunc("POST /settle", func(w http.ResponseWriter, r *http.Request) {
		var call struct {
			Owner, Call string
			Tokens      *int64
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048)).Decode(&call) != nil {
			w.WriteHeader(400)
			return
		}
		if _, err := db.Exec(r.Context(), q.SettlePlanDraft, call.Owner, call.Call, call.Tokens); err != nil {
			w.WriteHeader(503)
			return
		}
		w.WriteHeader(204)
	})
	addr := os.Getenv("EPISODE_BUDGET_BIND")
	if addr == "" {
		addr = "127.0.0.1:14328"
	}
	server := http.Server{Addr: addr, Handler: mux, ReadTimeout: 5 * time.Second, WriteTimeout: 10 * time.Second}
	go func() { <-ctx.Done(); server.Close() }()
	return server.ListenAndServe()
}
