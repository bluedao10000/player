UPDATE knowledge_sync_runs SET finished_at=now(),status='success',remote_version=$2,download_url=$3,http_status=$4,content_length=$5,checksum=$6,
   records_read=$7,records_created=(SELECT count(*) FROM incoming WHERE action='created'),records_updated=(SELECT count(*) FROM incoming WHERE action='updated'),
   records_unchanged=(SELECT count(*) FROM incoming WHERE action='unchanged'),records_deleted=(SELECT count(*) FROM removed) WHERE id=$1
