UPDATE episode_capture_attempts a SET finished_at=now(),error=nullif(left($4,1000),'') FROM episode_plans p
   WHERE a.id=$1 AND a.topic=p.topic AND p.world_slug=$2 AND p.spec=$3 AND a.finished_at IS NULL
