UPDATE knowledge_sync_runs SET status='success',finished_at=now(),skipped=true,skip_reason='version_unchanged',remote_version=$2,checksum=$2,records_read=1,records_unchanged=1 WHERE id=$1
