package knowledge

import (
	"context"
	"crypto/sha256"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"io"
	"os"
	"path/filepath"
	"regexp"
)

var digestPattern = regexp.MustCompile("^sha256:[0-9a-f]{64}$")

func archivePath(key string) (string, error) {
	if !digestPattern.MatchString(key) {
		return "", fmt.Errorf("invalid snapshot checksum")
	}
	dir := os.Getenv("KNOWLEDGE_ARCHIVE_DIR")
	if dir == "" {
		base, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(base, "tidepost", "knowledge-archive")
	}
	return filepath.Join(dir, key[7:]), nil
}
func archive(f *os.File, key string) error {
	path, e := archivePath(key)
	if e != nil {
		return e
	}
	dir := filepath.Dir(path)
	if e = os.MkdirAll(dir, 0700); e != nil {
		return e
	}
	tmp, e := os.CreateTemp(dir, ".snapshot-")
	if e != nil {
		return e
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	if _, e = f.Seek(0, 0); e != nil {
		return e
	}
	if _, e = io.Copy(tmp, f); e != nil {
		return e
	}
	if e = tmp.Sync(); e != nil {
		return e
	}
	if e = tmp.Close(); e != nil {
		return e
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return err
	}
	parent, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer parent.Close()
	return parent.Sync()
}
func openArchive(key string, st *stats) (*os.File, error) {
	p, e := archivePath(key)
	if e != nil {
		return nil, e
	}
	f, e := os.Open(p)
	if e != nil {
		return nil, e
	}
	h := sha256.New()
	st.Length, e = io.Copy(h, f)
	st.Checksum = fmt.Sprintf("sha256:%x", h.Sum(nil))
	if e == nil && key != st.Checksum {
		e = fmt.Errorf("archive checksum mismatch")
	}
	if e == nil {
		_, e = f.Seek(0, 0)
	}
	if e != nil {
		f.Close()
		return nil, e
	}
	return f, nil
}
func Rebuild(ctx context.Context, db *pgxpool.Pool, name, version string) error {
	var id int
	if err := db.QueryRow(ctx, "SELECT id FROM knowledge_sources WHERE name=$1 AND enabled", name).Scan(&id); err != nil {
		return err
	}
	if version == "" {
		version = "@current"
	}
	return syncSource(ctx, db, id, true, true, version)
}
