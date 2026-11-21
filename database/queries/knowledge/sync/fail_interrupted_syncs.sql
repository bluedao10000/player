UPDATE knowledge_sync_runs SET status='failed',finished_at=now(),error_message='worker stopped before commit'
 WHERE source_id=$1 AND status IN ('running','pending')
