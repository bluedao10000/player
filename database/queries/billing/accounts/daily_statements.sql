SELECT coalesce(jsonb_agg(to_jsonb(s) ORDER BY id),'[]') FROM billing_statements s WHERE day=$1 AND currency=$2
