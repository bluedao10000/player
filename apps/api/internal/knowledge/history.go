package knowledge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DiffSnapshots compares the upstream document, including fields unused by normalization.
// A missing side denotes entry creation or withdrawal.
func DiffSnapshots(ctx context.Context, db *pgxpool.Pool, source, from, to, key string) (map[string][2]json.RawMessage, error) {
	if key == "" || from == "" || to == "" {
		return nil, fmt.Errorf("record key and two versions required")
	}
	records := [2]map[string]json.RawMessage{}
	for i, version := range []string{from, to} {
		var digest, kind, asset string
		err := db.QueryRow(ctx, `SELECT snapshot_key,parser_type,v.asset_name FROM knowledge_source_versions v JOIN knowledge_sources s ON s.id=v.source_id WHERE s.name=$1 AND v.version=$2 AND snapshot_key IS NOT NULL`, source, version).Scan(&digest, &kind, &asset)
		if err != nil {
			return nil, err
		}
		f, err := openArchive(digest, &stats{})
		if err != nil {
			return nil, err
		}
		found := errors.New("record found")
		emit := func(id string, raw json.RawMessage, _ normalized) error {
			if e := ctx.Err(); e != nil {
				return e
			}
			if id != key {
				return nil
			}
			if e := json.Unmarshal(raw, &records[i]); e != nil {
				return e
			}
			return found
		}
		switch kind {
		case "github-jsonl":
			_, err = parseAOD(f, emit)
		case "npm-dist":
			_, err = parseBangumi(f, asset, emit)
		default:
			err = fmt.Errorf("unsupported archive format")
		}
		f.Close()
		if err != nil && !errors.Is(err, found) {
			return nil, err
		}
	}
	if records[0] == nil && records[1] == nil {
		return nil, pgx.ErrNoRows
	}
	result := map[string][2]json.RawMessage{}
	for _, record := range records {
		for k := range record {
			a, b := records[0][k], records[1][k]
			if !equivalentJSON(a, b) {
				result[k] = [2]json.RawMessage{a, b}
			}
		}
	}
	return result, nil
}

func Reproject(ctx context.Context, db *pgxpool.Pool, source string) error {
	return pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(82045102); CREATE TEMP TABLE affected(work_id uuid PRIMARY KEY) ON COMMIT DROP"); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, "INSERT INTO affected SELECT DISTINCT work_id FROM knowledge_source_records r JOIN knowledge_sources s ON s.id=r.source_id WHERE s.name=$1", source)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return fmt.Errorf("no source records")
		}
		_, err = tx.Exec(ctx, projectionSQL)
		return err
	})
}

func equivalentJSON(a, b json.RawMessage) bool {
	if bytes.Equal(a, b) {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	var left, right any
	for _, item := range []struct {
		raw  json.RawMessage
		dest *any
	}{{a, &left}, {b, &right}} {
		decoder := json.NewDecoder(bytes.NewReader(item.raw))
		decoder.UseNumber()
		if err := decoder.Decode(item.dest); err != nil {
			return false
		}
	}
	return reflect.DeepEqual(left, right)
}
