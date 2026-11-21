SELECT coalesce(jsonb_agg(to_jsonb(x) ORDER BY started_at DESC),'[]') FROM
 (SELECT id,started_at,finished_at,status,report FROM knowledge_quality_runs ORDER BY started_at DESC LIMIT $1)x
