SELECT coalesce(jsonb_agg(to_jsonb(x) ORDER BY seq DESC),'[]') FROM (
   SELECT * FROM model_usage_receipts WHERE started_at >= $1 AND started_at < $2 AND seq < $3 AND ($4='' OR subject=$4) ORDER BY seq DESC LIMIT $5)x
