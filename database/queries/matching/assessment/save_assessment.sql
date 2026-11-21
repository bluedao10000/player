INSERT INTO knowledge_ai_match_assessments AS a(candidate_id,evidence_hash,provider,model,prompt_version,evidence,recommendation,requested_by) VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING to_jsonb(a)
