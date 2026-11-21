SELECT (SELECT DISTINCT k.id FROM work_sources s JOIN works k ON k.id=s.work_id WHERE s.provider=$1 AND s.external_id=$2 AND k.active AND k.type=$4 AND k.season_year=$3)::text
