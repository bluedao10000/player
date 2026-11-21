SELECT coalesce(jsonb_agg(to_jsonb(x)),'[]') FROM (SELECT * FROM knowledge_match_candidates ORDER BY id LIMIT $1)x
