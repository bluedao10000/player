// Package modelusage records provider consumption independently of the business result.
package modelusage

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Event struct {
	IdempotencyKey string          `json:"idempotency_key,omitempty"`
	Account        string          `json:"account"`
	Size           string          `json:"size,omitempty"`
	Quality        string          `json:"quality,omitempty"`
	ID             string          `json:"id"`
	Purpose        string          `json:"purpose"`
	Operation      string          `json:"operation"`
	Subject        string          `json:"subject"`
	Endpoint       string          `json:"endpoint"`
	Model          string          `json:"model"`
	Started        time.Time       `json:"started"`
	Finished       *time.Time      `json:"finished,omitempty"`
	Outcome        string          `json:"outcome"`
	Status         int             `json:"status,omitempty"`
	Request        string          `json:"request,omitempty"`
	Response       string          `json:"response,omitempty"`
	Resolved       string          `json:"resolved,omitempty"`
	Usage          json.RawMessage `json:"usage,omitempty"`
	Images         *int            `json:"images,omitempty"`
}

func NewID() string {
	var b [16]byte
	rand.Read(b[:])
	b[6] = b[6]&15 | 64
	b[8] = b[8]&63 | 128
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func count(raw map[string]json.RawMessage, names ...string) *int64 {
	for _, key := range names {
		var n *int64
		if json.Unmarshal(raw[key], &n) == nil && n != nil && *n >= 0 {
			return n
		}
	}
	return nil
}

// Keep provider unit breakdowns but never store provider output or request payloads.
func usageFields(raw map[string]json.RawMessage, depth int) map[string]any {
	out := map[string]any{}
	for key, v := range raw {
		if len(key) > 64 {
			continue
		}
		if n := count(raw, key); n != nil {
			out[key] = *n
			continue
		}
		if depth < 2 {
			var sub map[string]json.RawMessage
			if json.Unmarshal(v, &sub) == nil && len(sub) > 0 {
				out[key] = usageFields(sub, depth+1)
			}
		}
	}
	return out
}

func Save(ctx context.Context, db *pgxpool.Pool, e Event) error {
	if e.Started.IsZero() || strings.TrimSpace(e.Subject) == "" || e.Operation == "" || e.Endpoint == "" || e.Model == "" {
		return errors.New("incomplete model call identity")
	}
	var raw map[string]json.RawMessage
	_ = json.Unmarshal(e.Usage, &raw)
	input, output, total := count(raw, "input_tokens", "prompt_tokens"), count(raw, "output_tokens", "completion_tokens"), count(raw, "total_tokens")
	if total == nil && input != nil && output != nil && *input <= math.MaxInt64-*output {
		n := *input + *output
		total = &n
	}
	if e.Account == "" {
		e.Account = "default"
	}
	var usage any
	if raw != nil {
		usage = usageFields(raw, 0)
	}
	tag, err := db.Exec(ctx, `INSERT INTO model_calls(id,purpose,operation,subject,endpoint,requested_model,started_at,finished_at,outcome,http_status,request_id,response_id,resolved_model,usage,input_tokens,output_tokens,total_tokens,images,provider_account,image_size,image_quality,idempotency_key)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,nullif($10,0),nullif($11,''),nullif($12,''),nullif($13,''),$14,$15,$16,$17,$18,$19,nullif($20,''),nullif($21,''),nullif($22,''))
 ON CONFLICT(id) DO UPDATE SET finished_at=excluded.finished_at,outcome=excluded.outcome,http_status=excluded.http_status,
 request_id=excluded.request_id,response_id=excluded.response_id,resolved_model=excluded.resolved_model,usage=excluded.usage,input_tokens=excluded.input_tokens,
 output_tokens=excluded.output_tokens,total_tokens=excluded.total_tokens,images=excluded.images
 WHERE model_calls.outcome='started' AND excluded.outcome<>'started'
 AND ROW(model_calls.purpose,model_calls.operation,model_calls.subject,model_calls.endpoint,model_calls.provider_account,model_calls.requested_model,model_calls.started_at)
 = ROW(excluded.purpose,excluded.operation,excluded.subject,excluded.endpoint,excluded.provider_account,excluded.requested_model,excluded.started_at)`, e.ID, e.Purpose, e.Operation, e.Subject, e.Endpoint, e.Model, e.Started, e.Finished, e.Outcome, e.Status, e.Request, e.Response, e.Resolved, usage, input, output, total, e.Images, e.Account, e.Size, e.Quality, e.IdempotencyKey)
	if err != nil || tag.RowsAffected() > 0 {
		return err
	}
	// A repeated terminal event or a late start is safe to acknowledge only
	// when its immutable identity (and any final receipt) agrees with storage.
	var identical bool
	err = db.QueryRow(ctx, `SELECT purpose=$2 AND operation=$3 AND subject=$4 AND endpoint=$5 AND provider_account=$6
 AND requested_model=$7 AND started_at=$8 AND ($9='started' OR
 (outcome=$9 AND ROW(request_id,response_id,resolved_model,usage,images,http_status,finished_at)
 IS NOT DISTINCT FROM ROW(nullif($10,''),nullif($11,''),nullif($12,''),$13::jsonb,$14::integer,nullif($15::integer,0),$16::timestamptz)))
 FROM model_calls WHERE id=$1`, e.ID, e.Purpose, e.Operation, e.Subject, e.Endpoint, e.Account, e.Model, e.Started, e.Outcome,
		e.Request, e.Response, e.Resolved, usage, e.Images, e.Status, e.Finished).Scan(&identical)
	if err == nil && !identical {
		return fmt.Errorf("conflicting receipt for model call %s", e.ID)
	}
	return err
}
