package knowledge

import (
	"context"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type source struct {
	ID                                          int
	Name, Kind, DataURL, CheckURL, Asset, Local string
}
type artifact struct {
	Version, URL, Digest string
	Size                 int64
}
type stats struct {
	Remote, URL, Checksum string
	HTTP, Read, Failed    int
	Length                int64
}

var client = &http.Client{Timeout: 6 * time.Minute}

func request(ctx context.Context, address string, auth bool) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", address, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "anime-knowledge-sync/1")
	if auth && req.URL.Hostname() == "api.github.com" {
		token := os.Getenv("KNOWLEDGE_GITHUB_TOKEN")
		if token == "" {
			token = os.Getenv("GITHUB_TOKEN")
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
	}
	return client.Do(req)
}
func check(ctx context.Context, s source, st *stats) (artifact, error) {
	var a artifact
	resp, err := request(ctx, s.CheckURL, true)
	if err != nil {
		return a, err
	}
	defer resp.Body.Close()
	st.HTTP = resp.StatusCode
	if resp.StatusCode != 200 {
		return a, fmt.Errorf("version check HTTP %d", resp.StatusCode)
	}
	dec := json.NewDecoder(io.LimitReader(resp.Body, 16<<20))
	switch s.Kind {
	case "github-jsonl":
		var release struct {
			Tag    string `json:"tag_name"`
			Assets []struct {
				Name, Digest string
				ID, Size     int64
			}
		}
		if err := dec.Decode(&release); err != nil {
			return a, err
		}
		for _, x := range release.Assets {
			if x.Name == s.Asset && x.ID > 0 && x.Size > 0 && strings.HasPrefix(x.Digest, "sha256:") {
				return artifact{fmt.Sprintf("%s/%d/%s", release.Tag, x.ID, x.Digest), s.DataURL, x.Digest, x.Size}, nil
			}
		}
		return a, fmt.Errorf("release lacks expected asset/digest %q", s.Asset)
	case "npm-dist":
		var pkg struct {
			Name, Version, Main string
			Dist                struct{ Tarball, Integrity string }
		}
		if err := dec.Decode(&pkg); err != nil {
			return a, err
		}
		target, err := url.Parse(pkg.Dist.Tarball)
		registry, e := url.Parse(s.DataURL)
		if err != nil || e != nil || target.Host == "" || target.Host != registry.Host || target.Scheme != registry.Scheme || pkg.Version == "" || pkg.Main != "dist/data.json" || !strings.HasPrefix(pkg.Dist.Integrity, "sha512-") {
			return a, fmt.Errorf("invalid npm package artifact")
		}
		return artifact{pkg.Version, target.String(), pkg.Dist.Integrity, 0}, nil
	default:
		return a, fmt.Errorf("unsupported source type %q", s.Kind)
	}
}

const maxDownload = 512 << 20

func download(ctx context.Context, a artifact, st *stats) (*os.File, error) {
	resp, err := request(ctx, a.URL, false)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	st.HTTP = resp.StatusCode
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("download HTTP %d", resp.StatusCode)
	}
	if resp.ContentLength > maxDownload {
		return nil, fmt.Errorf("asset exceeds %d bytes", maxDownload)
	}
	f, err := os.CreateTemp("", "anime-kb-*")
	if err != nil {
		return nil, err
	}
	ok := false
	defer func() {
		if !ok {
			f.Close()
			os.Remove(f.Name())
		}
	}()
	h := sha256.New()
	sri := sha512.New()
	n, err := io.Copy(io.MultiWriter(f, h, sri), io.LimitReader(resp.Body, maxDownload+1))
	st.Length = n
	st.Checksum = "sha256:" + hex.EncodeToString(h.Sum(nil))
	if err != nil {
		return nil, err
	}
	if n == 0 || n > maxDownload || (a.Size > 0 && n != a.Size) || (resp.ContentLength >= 0 && n != resp.ContentLength) {
		return nil, fmt.Errorf("asset length mismatch: %d", n)
	}
	if a.Digest != st.Checksum && a.Digest != "sha512-"+base64.StdEncoding.EncodeToString(sri.Sum(nil)) {
		return nil, fmt.Errorf("upstream digest mismatch")
	}
	if _, err = f.Seek(0, 0); err != nil {
		return nil, err
	}
	ok = true
	return f, nil
}
