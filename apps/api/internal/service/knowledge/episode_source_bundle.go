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
			Number                 int
			Title, Official, Guide string
		}
		Proofs struct{ Official, Guide []sourceRuleProof }
	}
	if err := json.NewDecoder(io.LimitReader(input, 65536)).Decode(&verified); err != nil {
		return 0, err
	}
	var rules sourceRules
	count := len(verified.Sources)
	if !planSlug.MatchString(world) || count < 2 || count > 5 || !shaRE.MatchString(verified.RuleSHA256) ||
		json.Unmarshal(verified.SourceRules, &rules) != nil || rules.Guide != "paired" ||
		len(verified.Proofs.Official) != count || len(verified.Proofs.Guide) != count {
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
		if source.Number < 1 || seen[source.Number] {
			return 0, errors.New("验证样本集号无效")
		}
		seen[source.Number] = true
		spec := episodeSpec{Number: source.Number, Official: source.Official, Guide: source.Guide,
			OfficialTitle: source.Title, GuideTitle: source.Title, SourceRules: &rules}
		if err := spec.validateRules(); err != nil {
			return 0, err
		}
		for _, proof := range []sourceRuleProof{verified.Proofs.Official[i], verified.Proofs.Guide[i]} {
			if proof.Episode != source.Number || !shaRE.MatchString(proof.PageSHA256) || !shaRE.MatchString(proof.TextSHA256) {
				return 0, errors.New("分集验证回执无法匹配来源")
			}
		}
	}
	var updated int64
	err := pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		for i, source := range verified.Sources {
			bundle := map[string]any{
				"number": source.Number, "official": source.Official, "guide": source.Guide,
				"officialTitle": source.Title, "guideTitle": source.Title, "sourceRules": verified.SourceRules,
				"ruleSha256": verified.RuleSHA256, "officialProof": verified.Proofs.Official[i], "guideProof": verified.Proofs.Guide[i],
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
