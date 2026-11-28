package modelusage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"anime-agent/api/internal/config"
)

const LegacyOutboxEnv = "KNOWLEDGE_LEGACY_USAGE_OUTBOX"

func recoveryDir() (string, error) {
	root, err := config.ArchiveDir()
	return filepath.Join(root, "model-usage-outbox"), err
}

func RecoveryDir() string {
	// Keep the historical cwd-relative write location when no user directory is available.
	dir, _ := recoveryDir()
	return dir
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

// ReplayConfigured checks both inputs even when one fails. A missing active
// outbox is normal unless the only recovery path is an uncertain cwd fallback.
func ReplayConfigured(ctx context.Context, db *pgxpool.Pool) error {
	active, rootErr := recoveryDir()
	legacy := os.Getenv(LegacyOutboxEnv)
	activeErr := replayDir(ctx, db, active, rootErr == nil || legacy != "")
	if legacy == "" {
		if rootErr != nil && errors.Is(activeErr, os.ErrNotExist) {
			return fmt.Errorf("%w (archive root: %v; set %s to the absolute historical outbox path)", activeErr, rootErr, LegacyOutboxEnv)
		}
		return activeErr
	}
	if !filepath.IsAbs(legacy) {
		return errors.Join(activeErr, fmt.Errorf("%s must be an absolute outbox path", LegacyOutboxEnv))
	}
	legacyErr := Replay(ctx, db, legacy)
	if legacyErr != nil {
		legacyErr = fmt.Errorf("%s: %w", LegacyOutboxEnv, legacyErr)
	}
	return errors.Join(activeErr, legacyErr)
}

// Replay imports an explicitly supplied directory. Its absence is an error.
func Replay(ctx context.Context, db *pgxpool.Pool, dir string) error {
	return replayDir(ctx, db, dir, false)
}

func replayDir(ctx context.Context, db *pgxpool.Pool, dir string, missingOK bool) error {
	files, err := os.ReadDir(dir)
	if missingOK && errors.Is(err, os.ErrNotExist) {
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
			return fmt.Errorf("decode %s: %w", path, err)
		}
		if err = Save(ctx, db, e); err != nil {
			return fmt.Errorf("save %s: %w", path, err)
		}
		if err = os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}
