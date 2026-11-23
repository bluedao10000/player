package knowledge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	queries "anime-agent/api/internal/repository/queries/knowledge"
)

// Sweep checks enabled, due sources; manual runs also probe sources not yet due.
func Sweep(ctx context.Context, db *pgxpool.Pool, name string, manual, refresh bool) error {
	rows, err := db.Query(ctx, queries.EnabledSources, name)
	if err != nil {
		return err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[int])
	if err != nil {
		return err
	}
	if name != "" && len(ids) == 0 {
		return fmt.Errorf("no enabled source %s", name)
	}
	var failures []error
	for _, id := range ids {
		if err := syncSource(ctx, db, id, manual, refresh, ""); err != nil {
			failures = append(failures, fmt.Errorf("source %d: %w", id, err))
		}
	}
	return errors.Join(failures...)
}
func syncSource(ctx context.Context, db *pgxpool.Pool, id int, manual, refresh bool, replay string) error {
	// A dedicated physical connection owns the session lock until Close, including cancellation.
	conn, err := pgx.ConnectConfig(ctx, db.Config().ConnConfig)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	var locked bool
	if err = conn.QueryRow(ctx, queries.TryLockSource, id).Scan(&locked); err != nil {
		return err
	}
	if !locked {
		if manual {
			_, err = conn.Exec(ctx, queries.RecordBusySkip, id)
			log.Printf("knowledge source=%d skipped: already running", id)
		}
		if replay != "" && err == nil {
			return fmt.Errorf("source is already running")
		}
		return err
	}
	// Only a dead worker can leave a running row while we hold its lock.
	_, err = conn.Exec(ctx, queries.FailInterruptedSyncs, id)
	if err != nil {
		return err
	}
	_, err = conn.Exec(ctx, queries.ClearFailedStaging, id)
	if err != nil {
		return err
	}
	var s source
	var due bool
	err = conn.QueryRow(ctx, queries.SourceConfiguration, id).
		Scan(&s.ID, &s.Name, &s.Kind, &s.DataURL, &s.CheckURL, &s.Asset, &s.Local, &due)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if !manual && !due {
		return nil
	}
	var batch string
	if err = conn.QueryRow(ctx, queries.StartSync, id, s.Local).Scan(&batch); err != nil {
		return err
	}
	var st stats
	_, err = conn.Exec(ctx, queries.TouchSource, id)
	if err == nil {
		switch s.Kind {
		case "world-original", "world-enrichment":
			err = importWorld(ctx, conn, s, batch, &st, replay)
			if err != nil {
				st.Failed = 1
			}
		default:
			err = importBatch(ctx, conn, s, batch, &st, refresh, replay)
		}
	}
	if err != nil {
		recordCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		auditErr := pgx.BeginFunc(recordCtx, db, func(tx pgx.Tx) error {
			if _, e := tx.Exec(recordCtx, queries.FailSync, batch, st.Remote, st.URL, st.HTTP, st.Length, st.Checksum, st.Read, st.Failed, err.Error()); e != nil {
				return e
			}
			_, e := tx.Exec(recordCtx, queries.DeleteBatch, batch)
			return e
		})
		return errors.Join(err, auditErr)
	}
	log.Printf("knowledge source=%s run=%s version=%s records=%d", s.Name, batch, st.Remote, st.Read)
	return nil
}
func importBatch(ctx context.Context, c *pgx.Conn, s source, batch string, st *stats, refresh bool, replay string) error {
	var a artifact
	var err error
	if replay != "" {
		err = c.QueryRow(ctx, queries.ArchivedSourceVersion, s.ID, replay, s.Local).Scan(&a.Version, &a.Digest, &s.Kind, &s.Asset)
		a.URL = "archive:" + a.Digest
	} else {
		a, err = check(ctx, s, st)
	}
	if err != nil {
		return err
	}
	st.Remote = a.Version
	st.URL = a.URL
	var archived bool
	if err = c.QueryRow(ctx, queries.HasArchivedVersion, s.ID, a.Version).Scan(&archived); err != nil {
		return err
	}
	if a.Version == s.Local && !refresh && archived {
		_, err = c.Exec(ctx, queries.SkipUnchangedSource, batch, a.Version, a.URL, st.HTTP)
		return err
	}
	if _, err = c.Exec(ctx, queries.RecordDownload, batch, a.Version, a.URL, st.HTTP); err != nil {
		return err
	}
	var f *os.File
	if replay != "" {
		f, err = openArchive(a.Digest, st)
	} else {
		f, err = download(ctx, a, st)
	}
	if err != nil {
		return err
	}
	if replay == "" {
		defer os.Remove(f.Name())
	}
	defer f.Close()
	if _, err = c.Exec(ctx, queries.RecordChecksum, batch, st.Length, st.Checksum, st.HTTP); err != nil {
		return err
	}
	chunk := make([][]any, 0, 400)
	flush := func() error {
		if len(chunk) == 0 {
			return nil
		}
		err := pgx.BeginFunc(ctx, c, func(tx pgx.Tx) error {
			if _, err := tx.CopyFrom(ctx, pgx.Identifier{"staging_source_records"}, []string{"batch_id", "record_key", "raw", "norm"}, pgx.CopyFromRows(chunk)); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, queries.RecordReadCount, batch, st.Read)
			return err
		})
		chunk = chunk[:0]
		return err
	}
	emit := func(key string, raw json.RawMessage, n normalized) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		st.Read++
		sort.Slice(n.Titles, func(i, j int) bool {
			a, b := n.Titles[i], n.Titles[j]
			if a.Lang != b.Lang {
				return a.Lang < b.Lang
			}
			return a.Title < b.Title
		})
		norm, err := json.Marshal(n)
		if err != nil {
			return err
		}
		chunk = append(chunk, []any{batch, key, string(raw), string(norm)})
		if len(chunk) == cap(chunk) {
			return flush()
		}
		return nil
	}
	var meta map[string]any
	switch s.Kind {
	case "github-jsonl":
		meta, err = parseAOD(f, emit)
	case "npm-dist":
		meta, err = parseBangumi(f, s.Asset, emit)
	}
	if err != nil {
		st.Failed++
		return err
	}
	if err = flush(); err != nil {
		st.Failed++
		return err
	}
	if st.Read == 0 {
		return fmt.Errorf("empty snapshot")
	}
	if replay == "" {
		if err = archive(f, st.Checksum); err != nil {
			return err
		}
	}
	return pgx.BeginFunc(ctx, c, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, queries.LockProjection); err != nil {
			return err
		}
		var valid bool
		if err := tx.QueryRow(ctx, queries.ValidateStaging, batch, s.ID, st.Read).Scan(&valid); err != nil {
			return err
		}
		if !valid && replay == "" {
			return fmt.Errorf("snapshot count outside configured retention threshold")
		}
		var version int64
		if err := tx.QueryRow(ctx, queries.RegisterVersion, s.ID, a.Version, st.Checksum, meta, batch).Scan(&version); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, queries.BindSnapshot, version, st.Checksum, s.Kind, s.Asset); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, queries.CreateMergeContext); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, queries.SetMergeContext, batch, s.ID, version); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, queries.Merge); err != nil {
			return fmt.Errorf("merge: %w", err)
		}
		if replay != "" {
			if _, err := tx.Exec(ctx, queries.MarkReplayWorks); err != nil {
				return fmt.Errorf("merge: %w", err)
			}
		}
		if _, err := tx.Exec(ctx, queries.Projection); err != nil {
			return fmt.Errorf("merge: %w", err)
		}
		if replay == "" {
			if _, err := tx.Exec(ctx, queries.SaveSourceReviews); err != nil {
				return fmt.Errorf("source review: %w", err)
			}
		}
		if _, err := tx.Exec(ctx, queries.FinishSync, batch, a.Version, a.URL, st.HTTP, st.Length, st.Checksum, st.Read); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, queries.DeleteBatch, batch); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, queries.AdvanceSourceVersion, s.ID, a.Version)
		return err
	})
}
