SELECT purpose=$2 AND operation=$3 AND subject=$4 AND endpoint=$5 AND provider_account=$6
 AND requested_model=$7 AND started_at=$8 AND ($9='started' OR
 (outcome=$9 AND ROW(request_id,response_id,resolved_model,usage,images,http_status,finished_at)
 IS NOT DISTINCT FROM ROW(nullif($10,''),nullif($11,''),nullif($12,''),$13::jsonb,$14::integer,nullif($15::integer,0),$16::timestamptz)))
 FROM model_calls WHERE id=$1
