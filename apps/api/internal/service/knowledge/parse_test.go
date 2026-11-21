package knowledge

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"
)

func TestParseAOD(t *testing.T) {
	n := 0
	_, err := parseAOD(bytes.NewReader(lines(fixtureAnime(10, "First"))), func(key string, raw json.RawMessage, w normalized) error {
		n++
		if key != "mal:10" || w.Title != "First" || w.Year != 2020 {
			t.Fatal(key, w)
		}
		return nil
	})
	if err != nil || n != 1 {
		t.Fatal(n, err)
	}
	x := fixtureAnime(10, "Bad")
	delete(x, "animeSeason")
	_, err = parseAOD(bytes.NewReader(lines(x)), func(string, json.RawMessage, normalized) error { t.Fatal("invalid row emitted"); return nil })
	if err == nil {
		t.Fatal("invalid row accepted")
	}
}
func fixtureAnime(id int, name string) map[string]any {
	return map[string]any{"sources": []string{fmt.Sprintf("https://myanimelist.net/anime/%d", id)}, "title": name, "type": "TV", "episodes": 12, "status": "FINISHED", "animeSeason": map[string]any{"season": "SPRING", "year": 2020}, "picture": "https://example.org/p", "thumbnail": "https://example.org/t", "synonyms": []string{}, "relatedAnime": []string{}, "studios": []string{"Studio"}, "producers": []string{}, "tags": []string{"drama"}}
}
func lines(entries ...map[string]any) []byte {
	var b bytes.Buffer
	e := json.NewEncoder(&b)
	e.Encode(map[string]any{"$schema": "https://example.org/schema", "lastUpdate": "2020-01-01",
		"repository": "https://github.com/cedya77/anime-offline-database", "scoreRange": map[string]int{"minInclusive": 1, "maxInclusive": 10},
		"license": map[string]string{"name": "Open Data Commons Open Database License (ODbL) v1.0 + Database Contents License (DbCL) v1.0", "url": "https://example.org/license"}})
	for _, x := range entries {
		e.Encode(x)
	}
	return b.Bytes()
}
