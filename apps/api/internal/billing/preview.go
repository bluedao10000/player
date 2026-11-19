package billing

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Orders bind the creative brief and dimensions before any balance is reserved.
type ArtBrief struct {
	Subject      string `json:"original_subject" binding:"required,max=500"`
	VisualBible  string `json:"visual_bible" binding:"required,max=1000"`
	Scene        string `json:"scene" binding:"required,max=1000"`
	Composition  string `json:"composition" binding:"required,max=1000"`
	ArtDirection string `json:"art_direction" binding:"required,max=1000"`
}
type PreviewRequest struct {
	Brief ArtBrief `json:"brief" binding:"required"`
	Size  string   `json:"size" binding:"required,oneof=1024x1024 1024x1536 1536x1024"`
}

func ReadRequest(ctx context.Context, db *pgxpool.Pool, id string) (string, PreviewRequest, error) {
	var req PreviewRequest
	var raw []byte
	var canonical string
	err := db.QueryRow(ctx, "SELECT id::text,request FROM billing_orders WHERE id=$1", id).Scan(&canonical, &raw)
	if err == nil {
		err = json.Unmarshal(raw, &req)
	}
	return canonical, req, err
}

// Path is only called with order UUIDs read back from PostgreSQL.
func PreviewPath(id string) string {
	root := os.Getenv("KNOWLEDGE_ARCHIVE_DIR")
	if root == "" {
		root = "./data/knowledge-archive"
	}
	return filepath.Join(root, "billing-previews", id+".png")
}

// Publish by linking a flushed temporary file. A restarted worker can identify
// a completed delivery without making a second provider call.
func StorePreview(id, source string) error {
	path := PreviewPath(id)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	src, err := os.Open(source)
	if err != nil {
		return err
	}
	defer src.Close()
	tmp, err := os.CreateTemp(dir, ".preview-")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err = io.Copy(tmp, src); err == nil {
		err = tmp.Sync()
	}
	err = errors.Join(err, tmp.Close())
	if err != nil {
		return err
	}
	if err = os.Link(tmp.Name(), path); err != nil {
		return err
	}
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}

func FinalizePreview(ctx context.Context, db *pgxpool.Pool, id, actor string) error {
	var state, canonical, expectedSHA string
	if err := db.QueryRow(ctx, "SELECT id::text,status,coalesce(delivery_sha256,'') FROM billing_orders WHERE id=$1", id).Scan(&canonical, &state, &expectedSHA); err != nil {
		return err
	}
	if state == "running" || state == "delivered" {
		f, err := os.Open(PreviewPath(canonical))
		if errors.Is(err, os.ErrNotExist) {
			return ErrPending
		}
		if err != nil {
			return err
		}
		h := sha256.New()
		_, err = io.Copy(h, f)
		err = errors.Join(err, f.Close())
		if err != nil {
			return err
		}
		actual := hex.EncodeToString(h.Sum(nil))
		if state == "delivered" && actual != expectedSHA {
			return ErrPending
		}
		if state == "running" {
			if err = Deliver(ctx, db, canonical, actual); errors.Is(err, ErrConflict) {
				// Another finalizer may already have settled this same artifact.
				var completed bool
				if err = db.QueryRow(ctx, "SELECT status IN ('settled','refunded') AND delivery_sha256=$2 FROM billing_orders WHERE id=$1", canonical, actual).Scan(&completed); err != nil {
					return err
				}
				if !completed {
					return ErrConflict
				}
				return nil
			} else if err != nil {
				return err
			}
		}
	}
	return Change(ctx, db, canonical, "", "settle", actor, "")
}

// Recover artifacts already produced and receipts arriving after the provider returned.
// The lease bounds polling of unfinished orders and never starts another generation.
func RecoverPreviews(ctx context.Context, db *pgxpool.Pool) error {
	rows, err := db.Query(ctx, `WITH due AS (SELECT id FROM billing_orders WHERE status IN ('running','delivered') AND next_check_at<=now()
 ORDER BY next_check_at,id FOR UPDATE SKIP LOCKED LIMIT 20)
 UPDATE billing_orders o SET next_check_at=now()+interval '1 minute' FROM due WHERE o.id=due.id RETURNING o.id::text`)
	if err != nil {
		return err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return err
	}
	var failures error
	for _, id := range ids {
		err = FinalizePreview(ctx, db, id, "billing-recovery")
		if err != nil && !errors.Is(err, ErrPending) && !errors.Is(err, ErrConflict) {
			failures = errors.Join(failures, err)
		}
	}
	return failures
}
