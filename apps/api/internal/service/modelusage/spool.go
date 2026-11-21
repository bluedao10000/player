package modelusage

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

func RecoveryDir() string {
	if dir := os.Getenv("KNOWLEDGE_ARCHIVE_DIR"); dir != "" {
		return filepath.Join(dir, "model-usage-outbox")
	}
	dir, _ := os.UserConfigDir()
	return filepath.Join(dir, "tidepost", "knowledge-archive", "model-usage-outbox")
}

// Store a recoverable receipt when a successful provider exchange outlives
// the database connection.
func Deliver(ctx context.Context, db *pgxpool.Pool, dir string, e Event) error {
	if db != nil {
		err := Save(ctx, db, e)
		if err == nil || e.Outcome == "started" {
			return err
		}
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(e.Usage, &fields) == nil && fields != nil {
		e.Usage, _ = json.Marshal(usageFields(fields, 0))
	} else {
		e.Usage = nil
	}
	body, err := json.Marshal(e)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".usage-")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(body); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil || closeErr != nil {
		return errors.Join(err, closeErr)
	}
	if err = os.Rename(f.Name(), filepath.Join(dir, e.ID+"-"+e.Outcome+".json")); err != nil {
		return err
	}
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}

func Replay(ctx context.Context, db *pgxpool.Pool, dir string) error {
	files, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, file := range files {
		if !file.Type().IsRegular() || !strings.HasSuffix(file.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, file.Name())
		b, err := os.ReadFile(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		var e Event
		if err = json.Unmarshal(b, &e); err != nil {
			return err
		}
		if err = Save(ctx, db, e); err != nil {
			return err
		}
		if err = os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}
