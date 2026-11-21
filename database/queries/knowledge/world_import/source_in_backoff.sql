SELECT EXISTS(SELECT FROM world_source_policy WHERE source_id=$1 AND retry_after>now())
