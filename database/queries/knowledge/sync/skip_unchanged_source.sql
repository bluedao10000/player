UPDATE knowledge_sync_runs SET status='success',skipped=true,skip_reason='version_unchanged',finished_at=now(),remote_version=$2,download_url=$3,http_status=$4 WHERE id=$1
