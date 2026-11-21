SELECT EXISTS(SELECT FROM work_worlds w JOIN works k ON k.id=w.work_id
   JOIN work_sources s ON s.work_id=k.id WHERE w.slug=$1 AND s.provider=$2 AND s.external_id=$3 AND k.type=$4 AND k.season_year=$5)
