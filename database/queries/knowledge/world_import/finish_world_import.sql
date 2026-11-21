UPDATE knowledge_sync_runs SET status='success',finished_at=now(),remote_version=$2,checksum=$2,download_url=$3,
    content_length=$4,http_status=nullif($5,0),records_read=1,records_created=(local_version IS NULL)::int,records_updated=(local_version IS NOT NULL)::int WHERE id=$1
