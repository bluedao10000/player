package modelusage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Delivery struct {
	ID    string `json:"event_id"`
	Event Event  `json:"event"`
}

// Acknowledge only after all receipts have committed. Worker/Agent restarts
// and multiple consumers can replay either half of a call without adding usage.
func Pull(ctx context.Context, db *pgxpool.Pool, base string) error {
	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	url := strings.TrimRight(base, "/") + "/internal/model-usage"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return fmt.Errorf("usage outbox HTTP %d", res.StatusCode)
	}
	var batch []Delivery
	if err = json.NewDecoder(io.LimitReader(res.Body, 2<<20)).Decode(&batch); err != nil {
		return err
	}
	if len(batch) == 0 {
		return nil
	}
	if len(batch) > 50 {
		return fmt.Errorf("invalid usage batch size")
	}
	ids := make([]string, 0, len(batch))
	for _, d := range batch {
		if err = Save(ctx, db, d.Event); err != nil {
			return err
		}
		ids = append(ids, d.ID)
	}
	data, err := json.Marshal(ids)
	if err != nil {
		return err
	}
	ack, err := http.NewRequestWithContext(ctx, "POST", url+"/ack", bytes.NewReader(data))
	if err != nil {
		return err
	}
	ack.Header.Set("Content-Type", "application/json")
	res, err = http.DefaultClient.Do(ack)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != 204 {
		return fmt.Errorf("usage acknowledgement HTTP %d", res.StatusCode)
	}
	return nil
}
