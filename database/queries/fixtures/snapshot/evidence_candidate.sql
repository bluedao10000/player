SELECT (candidates->0->>'record_id')::bigint FROM knowledge_match_evidence WHERE id=$1
