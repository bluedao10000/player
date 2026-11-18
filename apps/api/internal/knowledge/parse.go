package knowledge

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"path"
	"sort"
	"strings"
	"time"

	schema "github.com/santhosh-tekuri/jsonschema/v6"
)

// Vendored from cedya77/anime-offline-database schemas/anime-offline-database.jsonl.schema.json.
//
//go:embed aod.schema.json
var aodSpec []byte

//go:embed aod.metadata.schema.json
var metadataSpec []byte

type ref struct {
	Provider string `json:"provider"`
	ID       string `json:"id"`
	URL      string `json:"url"`
}
type title struct {
	Lang  string `json:"lang"`
	Title string `json:"title"`
}
type normalized struct {
	Title      string           `json:"title"`
	Synopsis   string           `json:"synopsis,omitempty"`
	Original   string           `json:"original,omitempty"`
	Type       string           `json:"type"`
	Episodes   *int             `json:"episodes,omitempty"`
	Status     string           `json:"status,omitempty"`
	Season     string           `json:"season,omitempty"`
	Year       int              `json:"year,omitempty"`
	Start      string           `json:"start,omitempty"`
	End        string           `json:"end,omitempty"`
	Titles     []title          `json:"titles,omitempty"`
	Refs       []ref            `json:"refs,omitempty"`
	Tags       []string         `json:"tags,omitempty"`
	Studios    []string         `json:"studios,omitempty"`
	Producers  []string         `json:"producers,omitempty"`
	Related    []string         `json:"related,omitempty"`
	Broadcasts []map[string]any `json:"broadcasts,omitempty"`
}
type emitRecord func(string, json.RawMessage, normalized) error

func compileSchema(spec []byte) (*schema.Schema, error) {
	doc, err := schema.UnmarshalJSON(bytes.NewReader(spec))
	if err != nil {
		return nil, err
	}
	c := schema.NewCompiler()
	c.AssertFormat()
	if err = c.AddResource("mem://aod/schema", doc); err != nil {
		return nil, err
	}
	return c.Compile("mem://aod/schema")
}
func providerRef(raw string) (ref, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" {
		return ref{}, fmt.Errorf("invalid provider URL %q", raw)
	}
	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	aliases := map[string]string{"myanimelist.net": "mal", "anilist.co": "anilist", "anidb.net": "anidb", "kitsu.app": "kitsu", "kitsu.io": "kitsu", "bangumi.tv": "bangumi", "bgm.tv": "bangumi"}
	id := path.Base(strings.TrimSuffix(u.Path, "/"))
	if queryID := u.Query().Get("id"); queryID != "" {
		id = queryID
	}
	name := host
	if alias, ok := aliases[host]; ok {
		name = alias
	}
	return ref{name, id, raw}, nil
}
func preferred(refs []ref) string {
	for _, provider := range []string{"bangumi", "mal", "anilist", "anidb", "kitsu", "tmdb"} {
		for _, r := range refs {
			if r.Provider == provider {
				return provider + ":" + r.ID
			}
		}
	}
	if len(refs) == 0 {
		return ""
	}
	return refs[0].Provider + ":" + refs[0].ID
}
func parseAOD(r io.Reader, emit emitRecord) (map[string]any, error) {
	valid, err := compileSchema(aodSpec)
	if err != nil {
		return nil, err
	}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 65536), 8<<20)
	if !sc.Scan() {
		return nil, fmt.Errorf("missing JSONL metadata")
	}
	var meta map[string]any
	if err := json.Unmarshal(sc.Bytes(), &meta); err != nil {
		return nil, err
	}
	header, err := compileSchema(metadataSpec)
	if err != nil {
		return nil, err
	}
	if err := header.Validate(meta); err != nil {
		return nil, fmt.Errorf("invalid AOD metadata: %w", err)
	}
	for line := 2; sc.Scan(); line++ {
		raw := sc.Bytes()
		var doc any
		if err := json.Unmarshal(raw, &doc); err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
		if err := valid.Validate(doc); err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
		var a struct {
			Title, Type, Status string
			Episodes            int
			AnimeSeason         struct {
				Season string
				Year   int
			}
			Sources, Synonyms, Tags, Studios, Producers, RelatedAnime []string
		}
		if err := json.Unmarshal(raw, &a); err != nil {
			return nil, err
		}
		n := normalized{Title: a.Title, Type: a.Type, Status: a.Status, Episodes: &a.Episodes, Season: a.AnimeSeason.Season, Year: a.AnimeSeason.Year, Tags: a.Tags, Studios: a.Studios, Producers: a.Producers, Related: a.RelatedAnime, Titles: []title{{"und", a.Title}}}
		for _, s := range a.Synonyms {
			if strings.TrimSpace(s) != "" {
				n.Titles = append(n.Titles, title{"und", s})
			}
		}
		for _, s := range a.Sources {
			p, err := providerRef(s)
			if err != nil {
				return nil, err
			}
			n.Refs = append(n.Refs, p)
		}
		sort.Slice(n.Refs, func(i, j int) bool { return n.Refs[i].URL < n.Refs[j].URL })
		key := preferred(n.Refs)
		if key == "" || strings.TrimSpace(a.Title) == "" {
			return nil, fmt.Errorf("line %d has no identity/title", line)
		}
		if err := emit(key, raw, n); err != nil {
			return nil, err
		}
	}
	return meta, sc.Err()
}

type siteMeta struct {
	Title, URLTemplate, Type string
	Regions                  []string
}
type bangumiItem struct {
	Title, Type, Lang string
	Begin, End        *string
	Broadcast         string
	TitleTranslate    map[string][]string
	Sites             []json.RawMessage
}

func parseBangumi(r io.Reader, entry string, emit emitRecord) (map[string]any, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	ar := tar.NewReader(gz)
	for {
		h, err := ar.Next()
		if err == io.EOF {
			return nil, fmt.Errorf("missing archive member %s", entry)
		}
		if err != nil {
			return nil, err
		}
		if h.Name != entry {
			continue
		}
		if h.Typeflag != tar.TypeReg || h.Size > 128<<20 {
			return nil, fmt.Errorf("invalid dist member")
		}
		var dist struct {
			SiteMeta map[string]siteMeta
			Items    []json.RawMessage
		}
		dec := json.NewDecoder(ar)
		if err = dec.Decode(&dist); err != nil {
			return nil, err
		}
		if dec.Decode(new(any)) != io.EOF {
			return nil, fmt.Errorf("trailing data in dist JSON")
		}
		if len(dist.SiteMeta) == 0 {
			return nil, fmt.Errorf("missing siteMeta")
		}
		for name, site := range dist.SiteMeta {
			if site.Title == "" || site.URLTemplate == "" {
				return nil, fmt.Errorf("site %s missing title or template", name)
			}
			switch site.Type {
			case "info", "onair", "resource":
			default:
				return nil, fmt.Errorf("unsupported site type %s", site.Type)
			}
		}
		for i, raw := range dist.Items {
			var a bangumiItem
			if err = json.Unmarshal(raw, &a); err != nil {
				return nil, err
			}
			key, n, err := normalizeBangumi(a, dist.SiteMeta)
			if err != nil {
				return nil, fmt.Errorf("item %d: %w", i, err)
			}
			if err = emit(key, raw, n); err != nil {
				return nil, err
			}
		}
		return map[string]any{"siteMeta": dist.SiteMeta, "license": "CC-BY-4.0"}, nil
	}
}
func normalizeBangumi(a bangumiItem, sites map[string]siteMeta) (string, normalized, error) {
	n := normalized{Title: a.Title, Original: a.Title, Type: strings.ToUpper(a.Type), Titles: []title{{a.Lang, a.Title}}}
	if n.Type == "WEB" {
		n.Type = "ONA"
	}
	if a.Title == "" || a.Lang == "" || a.Begin == nil || a.End == nil || a.Sites == nil || a.TitleTranslate == nil {
		return "", n, fmt.Errorf("missing required item fields")
	}
	switch n.Type {
	case "TV", "MOVIE", "OVA", "ONA":
	default:
		return "", n, fmt.Errorf("unknown type %q", a.Type)
	}
	for _, v := range []struct {
		src  string
		dest *string
	}{{*a.Begin, &n.Start}, {*a.End, &n.End}} {
		if v.src != "" {
			t, err := time.Parse(time.RFC3339, v.src)
			if err != nil {
				return "", n, err
			}
			*v.dest = t.UTC().Format(time.DateOnly)
		}
	}
	if n.Start != "" {
		t, _ := time.Parse(time.DateOnly, n.Start)
		n.Year = t.Year()
	}
	for lang, ts := range a.TitleTranslate {
		for _, s := range ts {
			if s != "" {
				n.Titles = append(n.Titles, title{lang, s})
			}
		}
	}
	if *a.Begin != "" {
		n.Broadcasts = append(n.Broadcasts, map[string]any{"site": "original", "begin": *a.Begin, "end": *a.End, "broadcast": a.Broadcast})
	}
	for _, raw := range a.Sites {
		var s struct{ Site, ID, URL string }
		if err := json.Unmarshal(raw, &s); err != nil {
			return "", n, err
		}
		meta, ok := sites[s.Site]
		if !ok {
			return "", n, fmt.Errorf("undefined site %q", s.Site)
		}
		address := s.URL
		if address == "" && s.ID != "" {
			address = strings.ReplaceAll(meta.URLTemplate, "{{id}}", s.ID)
		}
		if meta.Type == "info" && s.ID != "" {
			n.Refs = append(n.Refs, ref{strings.ToLower(s.Site), s.ID, address})
		}
		if meta.Type == "onair" {
			var detail map[string]any
			if err := json.Unmarshal(raw, &detail); err != nil {
				return "", n, err
			}
			detail["url"] = address
			if _, ok := detail["regions"]; !ok {
				detail["regions"] = meta.Regions
			}
			n.Broadcasts = append(n.Broadcasts, detail)
		}
	}
	key := preferred(n.Refs)
	if key == "" {
		key = a.Title
	}
	// The dataset contains multiple releases/dubs of a single subject.
	sum := sha256.Sum256([]byte(strings.Join([]string{key, a.Lang, a.Type, *a.Begin}, "\x00")))
	return hex.EncodeToString(sum[:]), n, nil
}
