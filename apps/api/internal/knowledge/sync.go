package knowledge

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed merge.sql
var mergeSQL string

//go:embed projection.sql
var projectionSQL string

// Sweep checks enabled, due sources; manual runs also probe sources not yet due.
func Sweep(ctx context.Context, db *pgxpool.Pool, name string, manual, refresh bool) error {
	rows, err := db.Query(ctx, "SELECT id FROM knowledge_sources WHERE enabled AND ($1='' OR name=$1) ORDER BY id", name)
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
	if err = conn.QueryRow(ctx, "SELECT pg_try_advisory_lock(82045,$1)", id).Scan(&locked); err != nil {
		return err
	}
	if !locked {
		if manual {
			_, err = conn.Exec(ctx, "INSERT INTO knowledge_sync_runs(source_id,status,finished_at,local_version,skipped,skip_reason) SELECT id,'success',now(),current_version,true,'already_running' FROM knowledge_sources WHERE id=$1", id)
			log.Printf("knowledge source=%d skipped: already running", id)
		}
		if replay != "" && err == nil {
			return fmt.Errorf("source is already running")
		}
		return err
	}
	// Only a dead worker can leave a running row while we hold its lock.
	_, err = conn.Exec(ctx, `UPDATE knowledge_sync_runs SET status='failed',finished_at=now(),error_message='worker stopped before commit'
 WHERE source_id=$1 AND status IN ('running','pending')`, id)
	if err != nil {
		return err
	}
	_, err = conn.Exec(ctx, `DELETE FROM staging_source_records s USING knowledge_sync_runs r WHERE s.batch_id=r.id AND r.source_id=$1 AND r.status='failed'`, id)
	if err != nil {
		return err
	}
	var s source
	var due bool
	err = conn.QueryRow(ctx, `SELECT id,name,type,data_url,check_url,asset_name,coalesce(current_version,''),
 last_checked_at IS NULL OR last_checked_at + CASE WHEN (SELECT status FROM knowledge_sync_runs WHERE source_id=s.id ORDER BY started_at DESC LIMIT 1)='failed'
 THEN least(sync_interval,interval '15 minutes') ELSE sync_interval END<=now() FROM knowledge_sources s WHERE id=$1 AND enabled`, id).
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
	if err = conn.QueryRow(ctx, "INSERT INTO knowledge_sync_runs(source_id,status,local_version) VALUES($1,'running',nullif($2,'')) RETURNING id::text", id, s.Local).Scan(&batch); err != nil {
		return err
	}
	var st stats
	_, err = conn.Exec(ctx, "UPDATE knowledge_sources SET last_checked_at=now(),updated_at=now() WHERE id=$1", id)
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
			if _, e := tx.Exec(recordCtx, `UPDATE knowledge_sync_runs SET status='failed',finished_at=now(),remote_version=nullif($2,''),download_url=nullif($3,''),http_status=nullif($4,0),content_length=$5,checksum=nullif($6,''),records_read=$7,records_failed=$8,error_message=left($9,3000) WHERE id=$1 AND status='running'`, batch, st.Remote, st.URL, st.HTTP, st.Length, st.Checksum, st.Read, st.Failed, err.Error()); e != nil {
				return e
			}
			_, e := tx.Exec(recordCtx, "DELETE FROM staging_source_records WHERE batch_id=$1", batch)
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
		err = c.QueryRow(ctx, "SELECT version,checksum,parser_type,asset_name FROM knowledge_source_versions WHERE source_id=$1 AND version=coalesce(nullif($2,'@current'),$3) AND snapshot_key IS NOT NULL", s.ID, replay, s.Local).Scan(&a.Version, &a.Digest, &s.Kind, &s.Asset)
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
	if err = c.QueryRow(ctx, "SELECT EXISTS(SELECT FROM knowledge_source_versions WHERE source_id=$1 AND version=$2 AND snapshot_key IS NOT NULL)", s.ID, a.Version).Scan(&archived); err != nil {
		return err
	}
	if a.Version == s.Local && !refresh && archived {
		_, err = c.Exec(ctx, `UPDATE knowledge_sync_runs SET status='success',skipped=true,skip_reason='version_unchanged',finished_at=now(),remote_version=$2,download_url=$3,http_status=$4 WHERE id=$1`, batch, a.Version, a.URL, st.HTTP)
		return err
	}
	if _, err = c.Exec(ctx, "UPDATE knowledge_sync_runs SET remote_version=$2,download_url=$3,http_status=$4 WHERE id=$1", batch, a.Version, a.URL, st.HTTP); err != nil {
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
	if _, err = c.Exec(ctx, "UPDATE knowledge_sync_runs SET content_length=$2,checksum=$3,http_status=$4 WHERE id=$1", batch, st.Length, st.Checksum, st.HTTP); err != nil {
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
			_, err := tx.Exec(ctx, "UPDATE knowledge_sync_runs SET records_read=$2 WHERE id=$1", batch, st.Read)
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
		if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(82045102)"); err != nil {
			return err
		}
		var valid bool
		if err := tx.QueryRow(ctx, `SELECT (SELECT count(*) FROM staging_source_records WHERE batch_id=$1)=$3 AND
   $3::numeric >= (SELECT count(*) FROM knowledge_source_records WHERE source_id=$2 AND active)*(SELECT min_retained_ratio FROM knowledge_sources WHERE id=$2)`, batch, s.ID, st.Read).Scan(&valid); err != nil {
			return err
		}
		if !valid && replay == "" {
			return fmt.Errorf("snapshot count outside configured retention threshold")
		}
		var version int64
		if err := tx.QueryRow(ctx, `INSERT INTO knowledge_source_versions(source_id,version,checksum,metadata,run_id) VALUES($1,$2,$3,$4,$5)
  ON CONFLICT(source_id,version) DO UPDATE SET checksum=excluded.checksum WHERE knowledge_source_versions.checksum=excluded.checksum RETURNING id`, s.ID, a.Version, st.Checksum, meta, batch).Scan(&version); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, "UPDATE knowledge_source_versions SET snapshot_key=$2,parser_type=$3,asset_name=$4 WHERE id=$1", version, st.Checksum, s.Kind, s.Asset); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, "CREATE TEMP TABLE merge_context(batch uuid,source int, version bigint) ON COMMIT DROP"); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, "INSERT INTO merge_context VALUES($1,$2,$3)", batch, s.ID, version); err != nil {
			return err
		}
		projection := projectionSQL
		if replay != "" {
			projection = "INSERT INTO affected SELECT work_id FROM knowledge_source_records WHERE source_id=(SELECT source FROM merge_context) ON CONFLICT DO NOTHING;" + projection
		}
		if _, err := tx.Exec(ctx, mergeSQL+projection); err != nil {
			return fmt.Errorf("merge: %w", err)
		}
		if _, err := tx.Exec(ctx, `UPDATE knowledge_sync_runs SET finished_at=now(),status='success',remote_version=$2,download_url=$3,http_status=$4,content_length=$5,checksum=$6,
   records_read=$7,records_created=(SELECT count(*) FROM incoming WHERE action='created'),records_updated=(SELECT count(*) FROM incoming WHERE action='updated'),
   records_unchanged=(SELECT count(*) FROM incoming WHERE action='unchanged'),records_deleted=(SELECT count(*) FROM removed) WHERE id=$1`, batch, a.Version, a.URL, st.HTTP, st.Length, st.Checksum, st.Read); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, "DELETE FROM staging_source_records WHERE batch_id=$1", batch); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, "UPDATE knowledge_sources SET current_version=$2,last_synced_at=now(),updated_at=now() WHERE id=$1", s.ID, a.Version)
		return err
	})
}
