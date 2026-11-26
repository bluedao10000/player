package knowledge

import "fmt"

// sourceEvidence is the accepted shape of one episode's evidence. Validators and
// the writing chain read this table instead of testing the guide kind separately,
// so an evidence level is a row here, not another branch in seven functions.
type sourceEvidence struct {
	Documents int
	Citations int
	Narrative string // "guide" or "official"
}

var (
	pairedEvidence = sourceEvidence{Documents: 2, Citations: 5, Narrative: "guide"}
	singleEvidence = sourceEvidence{Documents: 1, Citations: 4, Narrative: "official"}
)

func evidenceFor(guide string) (sourceEvidence, error) {
	switch guide {
	case "", "paired", "wiki-episode", "paragraphs":
		return pairedEvidence, nil
	case "official_only":
		return singleEvidence, nil
	}
	return sourceEvidence{}, fmt.Errorf("未知的来源模式 %q", guide)
}

func (e *episodeSpec) evidence() (sourceEvidence, error) {
	if e == nil || e.SourceRules == nil {
		return pairedEvidence, nil
	}
	return evidenceFor(e.SourceRules.Guide)
}
