INSERT INTO knowledge_sync_runs(source_id,status,finished_at,local_version,skipped,skip_reason) SELECT id,'success',now(),current_version,true,'already_running' FROM knowledge_sources WHERE id=$1
