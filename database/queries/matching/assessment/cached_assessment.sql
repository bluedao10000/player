SELECT to_jsonb(a) FROM knowledge_ai_match_assessments a WHERE candidate_id=$1 AND evidence_hash=$2 AND provider=$3 AND model=$4 AND prompt_version=$5
