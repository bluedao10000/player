package knowledge

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"anime-agent/api/internal/aiprovider"
	"anime-agent/api/internal/modelusage"
)

func TestJevNativeProtocol(t *testing.T) {
	for _, tc := range []struct {
		name            string
		alias, distinct float64
		year            int
		want            string
	}{
		{"translated-title", .9, .1, 2010, "same"}, {"conflicting-date", .9, .1, 2020, "uncertain"}, {"sequel", .1, .91, 2020, "different"}, {"ambiguous", .53, .48, 2010, "uncertain"}, {"malformed", 2, .1, 2010, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v1/systemone" {
					t.Errorf("unexpected path %s", r.URL.Path)
				}
				var in struct {
					State     matchInput
					Questions map[string]any
					Model     string
				}
				if json.NewDecoder(r.Body).Decode(&in) != nil || len(in.Questions) != 2 || len(in.State.Candidates) != 1 || in.Model != "fixture" {
					t.Error("bad TypeSafe request")
				}
				fmt.Fprintf(w, `{"model":"resolved-fixture","answers":{"alternate_title":{"type":"noul","noul":%f},"distinct_installment":{"type":"noul","noul":%f}},"usage":{"input_tokens":120,"output_tokens":12}}`, tc.alias, tc.distinct)
			}))
			defer srv.Close()
			data := fmt.Sprintf(`{"incoming":{"title":"雪","year":2010,"type":"TV"},"candidates":[{"title":"Snow","year":%d,"type":"TV"}]}`, tc.year)
			var events []modelusage.Event
			client := aiprovider.Client{Base: srv.URL + "/v1", Model: "fixture", Purpose: "match", Subject: "match:1", Record: func(_ context.Context, e modelusage.Event) error { events = append(events, e); return nil }}
			result, err := jevAssessment(context.Background(), client, json.RawMessage(data))
			if len(events) != 2 || events[0].ID != events[1].ID || events[1].Resolved != "resolved-fixture" || string(events[1].Usage) != `{"input_tokens":120,"output_tokens":12}` {
				t.Fatalf("missing Jev receipt for %s", tc.name)
			}
			if tc.want == "" {
				if err == nil {
					t.Fatal("invalid probability accepted")
				}
				return
			}
			if err != nil || result.(map[string]any)["verdict"] != tc.want {
				t.Fatal(result, err)
			}
		})
	}
}
