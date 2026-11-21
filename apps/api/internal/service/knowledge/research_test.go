package knowledge

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// The model supplies only sentence numbers. It cannot inject a link, an excerpt
// or support from the wrong episode into the reader's preview.
func TestResearchEvidenceBoundaries(t *testing.T) {
	var docs []researchDocument
	var claims []researchClaim
	sentence := "A bounty hunter follows a new lead, which takes him across the city."
	for ep := 1; ep <= 2; ep++ {
		for _, src := range []string{"official", "guide"} {
			address := "https://www.cowboy-bebop.net/story/"
			if src == "guide" {
				address = fmt.Sprintf("https://en.wikipedia.org/wiki/List_of_Cowboy_Bebop_episodes#ep%d", ep)
			}
			docs = append(docs, researchDocument{ID: fmt.Sprintf("%s-%d", src, ep), Episode: ep, URL: address, Locator: "section", Text: sentence, Sentences: []string{sentence}, Fetched: time.Now(), SHA: strings.TrimPrefix(digest([]byte(sentence)), "sha256:")})
		}
		for i, kind := range []string{"motive", "turn", "cause", "cause"} {
			claims = append(claims, researchClaim{Episode: ep, Kind: kind, Text: "一份新的委托让赏金猎人重新开始追查。", Cites: []researchCitation{{ID: docs[(ep-1)*2+i%2].ID, Sentence: 1, URL: "https://invalid.example", Quote: "invented quote"}}})
		}
	}
	if err := validateDocuments(docs); err != nil {
		t.Fatal(err)
	}
	if err := validateClaims(claims, docs); err != nil {
		t.Fatal(err)
	}
	if claims[0].Cites[0].Quote != sentence || !strings.HasPrefix(claims[0].Cites[0].URL, docs[0].URL+"#:~:text=") {
		t.Fatal("unbound evidence")
	}
	claims[0].Cites[0].ID = "official-2"
	if validateClaims(claims, docs) == nil {
		t.Fatal("cross-episode reference accepted")
	}
	claims[0].Cites[0].ID = "official-1"
	claims[0].Cites[0].Sentence = 2
	if validateClaims(claims, docs) == nil {
		t.Fatal("missing sentence accepted")
	}
	docs[0].Text += " changed"
	if validateDocuments(docs) == nil {
		t.Fatal("altered source snapshot accepted")
	}
}
