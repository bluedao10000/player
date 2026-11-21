// Package aiprovider calls server-configured AI endpoints.
package aiprovider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"anime-agent/api/internal/service/modelusage"
)

type Client struct {
	Base, Model               string
	key                       string
	Purpose, Subject, Account string
	Record                    func(context.Context, modelusage.Event) error
}

func Config(prefix string) (Client, error) {
	c := Client{Base: strings.TrimRight(os.Getenv(prefix+"_BASE_URL"), "/"), Model: os.Getenv(prefix + "_MODEL"), Account: os.Getenv(prefix + "_ACCOUNT"), key: os.Getenv(prefix + "_API_KEY")}
	return validated(c, prefix)
}
func validated(c Client, prefix string) (Client, error) {
	u, err := url.Parse(c.Base)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "127.0.0.1" || u.Hostname() == "localhost" || u.Hostname() == "::1"))) || c.Model == "" {
		return Client{}, fmt.Errorf("configure %s_BASE_URL and %s_MODEL (HTTPS or local loopback)", prefix, prefix)
	}
	return c, nil
}
func TypeSafe() (Client, error) {
	key := os.Getenv("TYPESAFE_API_KEY")
	if key == "" {
		return Client{}, fmt.Errorf("configure TYPESAFE_API_KEY")
	}
	model := os.Getenv("TYPESAFE_MODEL")
	if model == "" {
		model = "jev-latest"
	}
	base := strings.TrimRight(os.Getenv("TYPESAFE_BASE_URL"), "/")
	if base == "" {
		base = "https://api.typesafe.ai/v1"
	}
	return validated(Client{Base: base, Model: model, Account: os.Getenv("TYPESAFE_ACCOUNT"), key: key}, "TYPESAFE")
}
func (c Client) Post(ctx context.Context, path string, input, output any, limit int64) (resultErr error) {
	body, err := json.Marshal(input)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", c.Base+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.key != "" {
		req.Header.Set("Authorization", "Bearer "+c.key)
	}
	client := http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	event := modelusage.Event{ID: modelusage.NewID(), Purpose: c.Purpose, Account: c.Account, Subject: c.Subject, Operation: strings.TrimPrefix(path, "/"), Endpoint: c.Base, Model: c.Model, Started: time.Now().UTC(), Outcome: "started"}
	if c.Record != nil {
		if err = c.Record(ctx, event); err != nil {
			return fmt.Errorf("record model dispatch: %w", err)
		}
		defer func() {
			now := time.Now().UTC()
			event.Finished = &now
			finish, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if e := c.Record(finish, event); e != nil {
				resultErr = errors.Join(resultErr, fmt.Errorf("record model receipt: %w", e))
			}
		}()
	}
	event.Outcome = "unknown"
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("provider unavailable: %w", err)
	}
	defer resp.Body.Close()
	event.Status = resp.StatusCode
	event.Request = resp.Header.Get("x-request-id")
	if resp.StatusCode/100 != 2 {
		event.Outcome = "http_error"
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return err
	}
	if int64(len(data)) > limit {
		return fmt.Errorf("provider response exceeds limit")
	}
	var receipt struct {
		ID, Model string
		Usage     json.RawMessage
	}
	if json.Unmarshal(data, &receipt) == nil {
		event.Response = receipt.ID
		event.Resolved = receipt.Model
		event.Usage = receipt.Usage
		if resp.StatusCode/100 == 2 {
			event.Outcome = "ok"
		}
	}
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("provider returned HTTP %d", resp.StatusCode)
	}
	return json.Unmarshal(data, output)
}
