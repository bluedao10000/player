// Package aiprovider calls server-configured AI endpoints.
package aiprovider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"strings"
)

type Client struct {
	Base, Model string
	key         string
}

func Config(prefix string) (Client, error) {
	c := Client{strings.TrimRight(os.Getenv(prefix+"_BASE_URL"), "/"), os.Getenv(prefix + "_MODEL"), os.Getenv(prefix + "_API_KEY")}
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
	return Client{Base: "https://api.typesafe.ai/v1", Model: model, key: key}, nil
}
func (c Client) Post(ctx context.Context, path string, input, output any, limit int64) error {
	body, err := json.Marshal(input)
	if err != nil {
		return err
	}
	return c.postBody(ctx, path, "application/json", body, output, limit)
}

// EditImage submits an actual reference file to the provider's image editing endpoint.
func (c Client) EditImage(ctx context.Context, fields map[string]string, reference []byte, output any, limit int64) error {
	mime := http.DetectContentType(reference)
	suffix, ok := map[string]string{"image/png": "png", "image/jpeg": "jpg", "image/webp": "webp"}[mime]
	if !ok {
		return fmt.Errorf("unsupported reference image type %s", mime)
	}
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			return err
		}
	}
	header := textproto.MIMEHeader{}
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="image"; filename="reference.%s"`, suffix))
	header.Set("Content-Type", mime)
	part, err := w.CreatePart(header)
	if err != nil {
		return err
	}
	if _, err = part.Write(reference); err != nil {
		return err
	}
	if err = w.Close(); err != nil {
		return err
	}
	return c.postBody(ctx, "/images/edits", w.FormDataContentType(), body.Bytes(), output, limit)
}
func (c Client) postBody(ctx context.Context, path, contentType string, body []byte, output any, limit int64) error {
	req, err := http.NewRequestWithContext(ctx, "POST", c.Base+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", contentType)
	if c.key != "" {
		req.Header.Set("Authorization", "Bearer "+c.key)
	}
	client := http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("provider unavailable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("provider returned HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return err
	}
	if int64(len(data)) > limit {
		return fmt.Errorf("provider response exceeds limit")
	}
	return json.Unmarshal(data, output)
}
