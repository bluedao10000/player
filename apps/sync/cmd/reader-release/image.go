package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const imageOrigin = "https://img.codeiy.com"

func publishImage(ctx context.Context, sha string, data []byte) error {
	base, err := url.Parse(os.Getenv("IMAGE_ADMIN_URL"))
	if err != nil || base.Host == "" || base.User != nil || (base.Scheme != "http" && base.Scheme != "https") ||
		(base.Path != "" && base.Path != "/") || base.RawQuery != "" || base.Fragment != "" {
		return fmt.Errorf("IMAGE_ADMIN_URL must be the private image service origin")
	}
	token := os.Getenv("IMAGE_API_TOKEN")
	if token == "" {
		return fmt.Errorf("IMAGE_API_TOKEN is required")
	}
	client := http.Client{
		Timeout:       2 * time.Minute,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	request := func(method, path, contentType string, body []byte) (*http.Response, error) {
		endpoint := strings.TrimRight(base.String(), "/") + path
		req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
			req.Header.Set("Authorization", "Bearer "+token)
		}
		return client.Do(req)
	}
	res, err := request("HEAD", "/i/"+sha, "", nil)
	if err != nil {
		return err
	}
	res.Body.Close()
	if res.StatusCode == http.StatusOK && res.Header.Get("ETag") == `"`+sha+`"` {
		return nil
	}
	if res.StatusCode != http.StatusNotFound {
		return fmt.Errorf("image readiness HTTP %d or checksum mismatch", res.StatusCode)
	}
	res, err = request("POST", "/v1/assets/upload?source=reader-release&name="+sha+".jpg", "image/jpeg", data)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		return fmt.Errorf("image upload HTTP %d", res.StatusCode)
	}
	var asset struct {
		ID  string
		SHA string `json:"sha256"`
	}
	if err := json.NewDecoder(res.Body).Decode(&asset); err != nil || asset.SHA != sha || asset.ID == "" {
		return fmt.Errorf("image upload returned an invalid asset")
	}
	res, err = request("POST", "/v1/assets/"+url.PathEscape(asset.ID)+"/publish", "application/json",
		[]byte(`{"reference":"reader-release:`+sha+`"}`))
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("image publication HTTP %d", res.StatusCode)
	}
	res, err = request("HEAD", "/i/"+sha, "", nil)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK || res.Header.Get("ETag") != `"`+sha+`"` {
		return fmt.Errorf("image publication HTTP %d or checksum mismatch", res.StatusCode)
	}
	return nil
}
