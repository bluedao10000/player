SELECT id,name,type,data_url,check_url,asset_name,coalesce(current_version,''),
 last_checked_at IS NULL OR last_checked_at + CASE WHEN (SELECT status FROM knowledge_sync_runs WHERE source_id=s.id ORDER BY started_at DESC LIMIT 1)='failed'
 THEN least(sync_interval,interval '15 minutes') ELSE sync_interval END<=now() FROM knowledge_sources s WHERE id=$1 AND enabled
