SELECT coalesce(jsonb_agg(to_jsonb(x) ORDER BY id),'[]') FROM
  (SELECT id,name,type,repository_url,enabled,sync_interval::text,last_checked_at,last_synced_at,current_version FROM knowledge_sources) x
