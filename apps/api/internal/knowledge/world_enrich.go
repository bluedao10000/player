package knowledge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"golang.org/x/net/html"
)

type cachedWorldDocument struct {
	etag, modified string
	body           []byte
}
type worldFetcher struct {
	conn     *pgx.Conn
	sourceID int
	changed  map[string]cachedWorldDocument
}

func (f *worldFetcher) get(ctx context.Context, address string, st *stats) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	st.URL = address
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Tidepost-Knowledge/1.0")
	var cached cachedWorldDocument
	err = f.conn.QueryRow(ctx, "SELECT etag,modified,body FROM world_fetch_cache WHERE source_id=$1 AND url=$2", f.sourceID, address).Scan(&cached.etag, &cached.modified, &cached.body)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if cached.etag != "" {
		req.Header.Set("If-None-Match", cached.etag)
	}
	if cached.modified != "" {
		req.Header.Set("If-Modified-Since", cached.modified)
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	st.HTTP = res.StatusCode
	if res.StatusCode == http.StatusNotModified && len(cached.body) > 0 {
		return cached.body, nil
	}
	if res.StatusCode == http.StatusTooManyRequests {
		until := time.Now().Add(15 * time.Minute)
		value := res.Header.Get("Retry-After")
		if seconds, e := strconv.ParseUint(value, 10, 31); e == nil {
			until = time.Now().Add(time.Duration(seconds) * time.Second)
		} else if date, e := http.ParseTime(value); e == nil {
			until = date
		}
		if _, e := f.conn.Exec(ctx, "UPDATE world_source_policy SET retry_after=$2 WHERE source_id=$1", f.sourceID, until); e != nil {
			return nil, e
		}
	}
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("%s: HTTP %d", address, res.StatusCode)
	}
	const max = 4 << 20
	raw, err := io.ReadAll(io.LimitReader(res.Body, max+1))
	if err == nil && (len(raw) == 0 || len(raw) > max) {
		err = fmt.Errorf("invalid document size at %s", address)
	}
	if err == nil {
		etag, modified := res.Header.Get("ETag"), res.Header.Get("Last-Modified")
		if etag != "" || modified != "" {
			f.changed[address] = cachedWorldDocument{etag, modified, raw}
		}
	}
	return raw, err
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}
func hasClass(n *html.Node, class string) bool {
	for _, s := range strings.Fields(attr(n, "class")) {
		if s == class {
			return true
		}
	}
	return false
}
func textOf(n *html.Node) string {
	var b strings.Builder
	for child := range n.Descendants() {
		if child.Type == html.TextNode {
			b.WriteString(child.Data)
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}
func officialEpisodes(raw []byte) (map[int]string, error) {
	root, err := html.Parse(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	out := map[int]string{}
	for box := range root.Descendants() {
		if !hasClass(box, "storybox") {
			continue
		}
		var num int
		var synopsis string
		for node := range box.Descendants() {
			if node.Data == "img" {
				parts := strings.Fields(attr(node, "alt"))
				if len(parts) > 0 {
					num, _ = strconv.Atoi(strings.TrimPrefix(parts[0], "#"))
				}
			}
			if hasClass(node, "storytxt") {
				synopsis = textOf(node)
			}
		}
		if num > 0 && synopsis != "" {
			if out[num] != "" {
				return nil, fmt.Errorf("duplicate official episode %d", num)
			}
			out[num] = synopsis
		}
	}
	return out, nil
}

type catalogEpisode struct {
	ID, Type int
	Subject  int `json:"subject_id"`
	Sort     float64
	Name     string
}

// The live sources must support the editorial reading before publishing its graph.
func enrichBebop(ctx context.Context, s source, w *worldManifest, st *stats, fetcher *worldFetcher) error {
	var chaseURL string
	if err := fetcher.conn.QueryRow(ctx, "SELECT supplement_url FROM world_source_policy WHERE source_id=$1", s.ID).Scan(&chaseURL); err != nil {
		return err
	}
	base := strings.TrimRight(s.DataURL, "/")
	episodesURL := base + "/episodes?subject_id=253&type=0&limit=2"
	castURL := base + "/subjects/253/characters"
	raw, err := fetcher.get(ctx, episodesURL, st)
	if err != nil {
		return err
	}
	var list struct {
		Data  []catalogEpisode
		Total int
	}
	if err = json.Unmarshal(raw, &list); err != nil {
		return err
	}
	if len(list.Data) != 2 || list.Total != 26 || len(w.Chapters) != 2 {
		return fmt.Errorf("unexpected episode catalog for subject 253")
	}
	raw, err = fetcher.get(ctx, castURL, st)
	if err != nil {
		return err
	}
	var people []struct {
		ID   int
		Name string
	}
	if err = json.Unmarshal(raw, &people); err != nil {
		return err
	}
	for i := range w.People {
		p := &w.People[i]
		found := 0
		for _, r := range people {
			if r.ID == p.ProviderID && strings.Contains(r.Name, p.Alias) {
				found++
				p.OriginalName = r.Name
			}
		}
		if found != 1 {
			return fmt.Errorf("unresolved character %s in subject 253", p.Slug)
		}
		w.Proofs["character/"+p.Slug] = worldProof{castURL, fmt.Sprintf("id=%d", p.ProviderID), digest([]byte(p.OriginalName))}
	}
	raw, err = fetcher.get(ctx, s.CheckURL, st)
	if err != nil {
		return err
	}
	official, err := officialEpisodes(raw)
	if err != nil {
		return err
	}
	required := [][]string{
		{"アシモフを追うスパイクとジェット", "アシモフは恋人のカテリーナと共に", "売人のバーテンの店を訪れる"},
		{"一匹の犬を巡ってチャイナタウン", "アインの登場秘話", "スパイク"},
	}
	for i := range w.Chapters {
		c := &w.Chapters[i]
		r := list.Data[i]
		if c.Episode != i+1 || r.Sort != float64(c.Episode) || r.Type != 0 || r.Subject != 253 || r.ID <= 0 || !strings.HasSuffix(r.Name, " "+c.Name) {
			return fmt.Errorf("episode %d identity conflict", i+1)
		}
		c.ProviderID = r.ID
		c.OriginalName = r.Name
		w.Proofs["catalog/"+c.Slug] = worldProof{episodesURL, fmt.Sprintf("episode id=%d", r.ID), digest([]byte(r.Name))}
		summary := official[c.Episode]
		for _, term := range required[i] {
			if !strings.Contains(summary, term) {
				return fmt.Errorf("unresolved official evidence for episode %d", c.Episode)
			}
		}
		w.Proofs["episode/"+c.Slug] = worldProof{s.CheckURL, fmt.Sprintf("storybox #%d .storytxt", c.Episode), digest([]byte(summary))}
	}
	raw, err = fetcher.get(ctx, chaseURL, st)
	if err != nil {
		return err
	}
	root, err := html.Parse(bytes.NewReader(raw))
	if err != nil {
		return err
	}
	for n := range root.Descendants() {
		if !hasClass(n, "shortSummaryText") {
			continue
		}
		summary := textOf(n)
		if strings.Contains(summary, "A bounty takes Spike and Jet to Mars") && strings.Contains(summary, "Hakim, Spike, and the group of scientists") && strings.Contains(summary, "give chase") {
			w.Proofs["chase-account"] = worldProof{chaseURL, "Stray Dog Strut · episode summary", digest([]byte(summary))}
			return nil
		}
	}
	return fmt.Errorf("episode 2 chase or Jet appearance unresolved")
}
