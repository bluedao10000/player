SELECT coalesce(jsonb_agg(to_jsonb(x) ORDER BY effective_at DESC,id),'[]') FROM
 (SELECT * FROM billing_rates WHERE ($1='' OR kind=$1) AND ($2='' OR currency=$2) ORDER BY effective_at DESC,id LIMIT 100)x
