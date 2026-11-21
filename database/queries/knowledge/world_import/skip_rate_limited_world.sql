UPDATE knowledge_sync_runs SET status='success',finished_at=now(),skipped=true,skip_reason='rate_limited' WHERE id=$1
