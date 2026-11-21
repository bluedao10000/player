SELECT jsonb_build_object('reason',reason,'incoming',incoming,'candidates',candidates) FROM knowledge_match_evidence WHERE id=$1 AND candidates IS NOT NULL
