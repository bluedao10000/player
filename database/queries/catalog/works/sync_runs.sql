SELECT coalesce(jsonb_agg(to_jsonb(x) ORDER BY started_at DESC),'[]') FROM
  (SELECT r.*,s.name AS source FROM knowledge_sync_runs r JOIN knowledge_sources s ON r.source_id=s.id WHERE $1='' OR s.name=$1 ORDER BY started_at DESC LIMIT $2)x
