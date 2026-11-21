UPDATE knowledge_quality_runs SET status='success',finished_at=now(),report=$2 WHERE id=$1
