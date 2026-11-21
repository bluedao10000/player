UPDATE knowledge_quality_runs SET status='failed',finished_at=now(),error_message=left($2,2000) WHERE id=$1
