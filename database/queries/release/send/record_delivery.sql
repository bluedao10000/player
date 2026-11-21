UPDATE reader_release_attempts SET finished_at=now(),error=$2,target_id=$3,href=$4 WHERE id=$1
