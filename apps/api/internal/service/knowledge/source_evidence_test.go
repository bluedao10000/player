package knowledge

import (
	"strings"
	"testing"
	"time"
)

// Each evidence level is one row of the table: paired needs two documents and
// five covered keys (three beat kinds plus both documents), official_only needs
// one official document and four.
func TestEvidenceLevels(t *testing.T) {
	cases := []struct {
		guide string
		want  sourceEvidence
	}{
		{"", pairedEvidence},
		{"paired", pairedEvidence},
		{"wiki-episode", pairedEvidence},
		{"paragraphs", pairedEvidence},
		{"official_only", singleEvidence},
	}
	for _, c := range cases {
		got, err := evidenceFor(c.guide)
		if err != nil || got != c.want {
			t.Fatalf("evidenceFor(%q) = %+v, %v; want %+v", c.guide, got, err, c.want)
		}
	}
	if _, err := evidenceFor("official"); err == nil {
		t.Fatal("unknown mode accepted")
	}
	if got, err := (&episodeSpec{}).evidence(); err != nil || got != pairedEvidence {
		t.Fatal("spec without rules must stay paired")
	}
}

const officialOnlyText = "The bounty hunter takes a new job, then learns the target is a friend, and the two men settle it without a bounty."

func officialOnlyPlan() episodeSpec {
	return episodeSpec{
		Number:        1,
		Official:      "https://www.cowboy-bebop.net/story/",
		OfficialTitle: "Asteroid Blues",
		SourceRules: &sourceRules{Guide: "official_only", Version: 1,
			OfficialRule: &episodeSourceRule{Kind: "json_episode_list", Episode: "n", Title: "t", Body: "b"}},
	}
}

func officialOnlyDoc(id string) researchDocument {
	return researchDocument{ID: id, Episode: 1, URL: "https://www.cowboy-bebop.net/story/", Locator: "ep1",
		Text: officialOnlyText, Sentences: []string{officialOnlyText}, Fetched: time.Now(),
		SHA: strings.TrimPrefix(digest([]byte(officialOnlyText)), "sha256:")}
}

func TestOfficialOnlyAcceptsOneSource(t *testing.T) {
	plan := officialOnlyPlan()
	if err := plan.validateRules(); err != nil {
		t.Fatal(err)
	}
	doc := officialOnlyDoc("official-1")
	if err := validateSourceDocuments([]researchDocument{doc}, 1, 1, &plan); err != nil {
		t.Fatal(err)
	}
	var claims []researchClaim
	for _, kind := range []string{"motive", "turn", "cause", "cause"} {
		claims = append(claims, researchClaim{Episode: 1, Kind: kind,
			Text:  "一份新的委托让赏金猎人重新开始追查。",
			Cites: []researchCitation{{ID: doc.ID, Sentence: 1}}})
	}
	if err := validateEpisodeClaims(claims, []researchDocument{doc}, 1, 1, singleEvidence); err != nil {
		t.Fatal(err)
	}
	if err := validateEpisodeClaims(claims[:2], []researchDocument{doc}, 1, 1, singleEvidence); err == nil {
		t.Fatal("short claim set accepted")
	}
	if err := validateSourceDocuments([]researchDocument{officialOnlyDoc("guide-1")}, 1, 1, &plan); err == nil {
		t.Fatal("official_only accepted a guide document")
	}
	if err := validateSourceDocuments(nil, 1, 1, &plan); err == nil {
		t.Fatal("empty source set accepted")
	}
}

func TestOfficialOnlyRejectsPairedRuleShape(t *testing.T) {
	plan := officialOnlyPlan()
	plan.Guide = "https://en.wikipedia.org/wiki/x"
	if plan.validateRules() == nil {
		t.Fatal("official_only accepted a guide source")
	}
	plan = officialOnlyPlan()
	plan.SourceRules.GuideRule = &episodeSourceRule{Kind: "html_episode_article", Heading: "h", Body: "b"}
	if plan.validateRules() == nil {
		t.Fatal("official_only accepted a guide rule")
	}
	plan = officialOnlyPlan()
	plan.OfficialTitle = ""
	if plan.validateRules() == nil {
		t.Fatal("official_only accepted a missing official title")
	}
}
