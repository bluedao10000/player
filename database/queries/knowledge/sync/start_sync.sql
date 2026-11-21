INSERT INTO knowledge_sync_runs(source_id,status,local_version) VALUES($1,'running',nullif($2,'')) RETURNING id::text
