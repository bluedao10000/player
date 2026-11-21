SELECT coalesce(jsonb_agg(to_jsonb(x) ORDER BY call_id),'[]') FROM
 (SELECT * FROM billing_daily_costs($1,$2) WHERE ($3='' OR call_id>nullif($3,'')::uuid)
 AND ($4='' OR provider_account=$4) AND ($5='' OR model=$5) ORDER BY call_id LIMIT 100)x
