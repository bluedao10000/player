SELECT coalesce(jsonb_agg(to_jsonb(x)),'[]')::text FROM (SELECT * FROM knowledge_match_candidates ORDER BY id LIMIT 100)x
