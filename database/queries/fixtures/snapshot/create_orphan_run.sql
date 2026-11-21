INSERT INTO knowledge_sync_runs(source_id,status) SELECT id,'running' FROM knowledge_sources WHERE name=$1 RETURNING id::text
