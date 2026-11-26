package knowledge

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"

	queries "anime-agent/api/internal/repository/queries/knowledge"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type sourceRuleProof struct {
	Episode                int
	PageSHA256, TextSHA256 string
}

// AttachEpisodeSourceBundle accepts the verified output of episode-source-cli.
// The production agent re-fetches each source and validates the locked rule at execution.
func AttachEpisodeSourceBundle(ctx context.Context, db *pgxpool.Pool, world string, input io.Reader) (int64, error) {
	var verified struct {
		SourceRules json.RawMessage
		RuleSHA256  string
		Sources     []struct {
			Number, SourceNumber   int
			Title, Official, Guide string
		}
		Proofs struct{ Official, Guide []sourceRuleProof }
	}
	if err := json.NewDecoder(io.LimitReader(input, 65536)).Decode(&verified); err != nil {
		return 0, err
	}
	var rules sourceRules
	count := len(verified.Sources)
	if !planSlug.MatchString(world) || count < 1 || count > 5 || !shaRE.MatchString(verified.RuleSHA256) ||
		json.Unmarshal(verified.SourceRules, &rules) != nil || (rules.Guide != "paired" && rules.Guide != "official_only") ||
		len(verified.Proofs.Official) != count {
		return 0, errors.New("采集规则或验证回执无效")
	}
	paired := rules.Guide == "paired"
	if paired != (len(verified.Proofs.Guide) == count) {
		return 0, errors.New("采集规则或验证回执无效")
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, verified.SourceRules); err != nil {
		return 0, err
	}
	digest := sha256.Sum256(compact.Bytes())
	if hex.EncodeToString(digest[:]) != verified.RuleSHA256 {
		return 0, errors.New("采集规则摘要与验证回执不一致")
	}
	seen := map[int]bool{}
	for i, source := range verified.Sources {
		// 续作的来源集号可以带偏移，回执按来源集号留证。
		if source.Number < 1 || source.SourceNumber < 0 || seen[source.Number] ||
			(!paired && source.Guide != "") {
			return 0, errors.New("验证样本集号无效")
		}
		seen[source.Number] = true
		proofNumber := source.Number
		if source.SourceNumber != 0 {
			proofNumber = source.SourceNumber
		}
		spec := episodeSpec{Number: source.Number, SourceNumber: source.SourceNumber, Official: source.Official, Guide: source.Guide,
			OfficialTitle: source.Title, GuideTitle: source.Title, SourceRules: &rules}
		if err := spec.validateRules(); err != nil {
			return 0, err
		}
		proofs := []sourceRuleProof{verified.Proofs.Official[i]}
		if paired {
			proofs = append(proofs, verified.Proofs.Guide[i])
		}
		for _, proof := range proofs {
			if proof.Episode != proofNumber || !shaRE.MatchString(proof.PageSHA256) || !shaRE.MatchString(proof.TextSHA256) {
				return 0, errors.New("分集验证回执无法匹配来源")
			}
		}
	}
	var updated int64
	err := pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		for i, source := range verified.Sources {
			bundle := map[string]any{
				"number": source.Number, "official": source.Official,
				"officialTitle": source.Title, "guideTitle": source.Title, "sourceRules": verified.SourceRules,
				"ruleSha256": verified.RuleSHA256, "officialProof": verified.Proofs.Official[i],
			}
			if paired {
				bundle["guide"], bundle["guideProof"] = source.Guide, verified.Proofs.Guide[i]
			}
			if source.SourceNumber != 0 {
				bundle["sourceNumber"] = source.SourceNumber
			}
			tag, err := tx.Exec(ctx, queries.Configuration["AttachSourceBundle"], world, source.Number, bundle)
			if err != nil {
				return err
			}
			updated += tag.RowsAffected()
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return updated, nil
}
