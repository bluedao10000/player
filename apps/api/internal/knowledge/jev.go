package knowledge

import (
	"context"
	"encoding/json"
	"errors"
	"os"

	"anime-agent/api/internal/aiprovider"
)

const jevVersion = "jev-anime-identity-1"

func matchProvider() (aiprovider.Client, string, error) {
	switch os.Getenv("MATCH_AI_PROVIDER") {
	case "", "openai":
		c, e := aiprovider.Config("MATCH_AI")
		return c, matchPromptVersion, e
	case "typesafe":
		c, e := aiprovider.TypeSafe()
		return c, jevVersion, e
	default:
		return aiprovider.Client{}, "", errors.New("unknown MATCH_AI_PROVIDER")
	}
}

type noul struct {
	Type        string   `json:"type"`
	Probability *float64 `json:"noul"`
}
type jevReply struct {
	Model   string          `json:"model"`
	Answers map[string]noul `json:"answers"`
	Usage   json.RawMessage `json:"usage"`
}

var jevQuestions = map[string]map[string]string{
	"alternate_title":      {"type": "noul", "instructions": "Do the incoming title and candidate titles identify alternate names for the same specific anime production? Translation and dubbing may change the title."},
	"distinct_installment": {"type": "noul", "instructions": "Does the incoming record identify a different installment from the candidate work? Look for season numbers, remake, compilation or film distinctions."},
}

// Source metadata can corroborate a title match. A disagreement blocks the
// positive recommendation, even when a language judgment sounds decisive.
func corroboration(input matchInput) (confirmed, conflict, sharedID bool) {
	var left []ref
	_ = json.Unmarshal(input.Incoming["provider_ids"], &left)
	for _, r := range input.Candidates {
		var right []ref
		_ = json.Unmarshal(r["provider_ids"], &right)
		for _, a := range left {
			for _, b := range right {
				if a.Provider != "" && a.Provider == b.Provider && a.ID != "" && b.ID != "" {
					if a.ID == b.ID {
						confirmed = true
						sharedID = true
					} else {
						conflict = true
					}
				}
			}
		}
		year := string(input.Incoming["year"])
		kind := string(input.Incoming["type"])
		if year != "" && year != "0" && year == string(r["year"]) && kind != "" && kind != `"UNKNOWN"` && kind == string(r["type"]) {
			confirmed = true
		}
		for _, field := range []string{"year", "type"} {
			a, b := string(input.Incoming[field]), string(r[field])
			if a != "" && b != "" && a != "0" && b != "0" && a != `"UNKNOWN"` && b != `"UNKNOWN"` && a != b {
				conflict = true
			}
		}
	}
	return
}
func jevAssessment(ctx context.Context, c aiprovider.Client, raw json.RawMessage) (any, error) {
	var input matchInput
	if err := json.Unmarshal(raw, &input); err != nil {
		return nil, err
	}
	request := map[string]any{"model": c.Model, "state": input, "questions": jevQuestions}
	var reply jevReply
	if err := c.Post(ctx, "/systemone", request, &reply, 64000); err != nil {
		return nil, err
	}
	if reply.Model == "" {
		return nil, errors.New("TypeSafe response missing model")
	}
	for name := range jevQuestions {
		v := reply.Answers[name]
		if v.Type != "noul" || v.Probability == nil || *v.Probability < 0 || *v.Probability > 1 {
			return nil, errors.New("invalid TypeSafe noul answer")
		}
	}
	alias, distinct := *reply.Answers["alternate_title"].Probability, *reply.Answers["distinct_installment"].Probability
	support, conflict, sharedID := corroboration(input)
	verdict, reason := "uncertain", "证据不足或信号冲突，需核对发行记录"
	if distinct >= 0.8 && alias <= 0.2 && !sharedID {
		verdict, reason = "different", "作品名称显示独立篇章或版本"
	}
	if alias >= 0.8 && distinct <= 0.2 && support && !conflict {
		verdict, reason = "same", "别名判断得到发行信息或来源 ID 佐证"
	}
	return map[string]any{"verdict": verdict, "reason": reason, "corroborated": support, "metadata_conflict": conflict, "request": request, "response": reply}, nil
}
