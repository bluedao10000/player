WITH occupied AS (
 SELECT range_agg(tstzrange(starts_at,ends_at,'[)')) periods FROM reader_paid_periods WHERE reader_id=$2 AND ends_at>$7
), term AS (
 SELECT greatest($7::timestamptz,max(upper(period))) begins FROM occupied,unnest(periods) period WHERE period @> $7::timestamptz
)
 INSERT INTO reader_store_purchases(transaction_id,reader_id,environment,bundle_id,product_id,plan,credits,purchased_at,starts_at,ends_at,revoked_at,signed_at,currency,price_milli,refund_event_at)
 SELECT $1,$2,$3,$4,$5,p.id,p.credits,$7,term.begins,term.begins+interval '720 hours',$8,$9,$10,$11,CASE WHEN $8::timestamptz IS NOT NULL OR $12 THEN $9::timestamptz END FROM reader_plans p,term WHERE p.id=$6
 ON CONFLICT(transaction_id) DO UPDATE SET
 revoked_at=CASE WHEN excluded.refund_event_at IS NOT NULL AND (reader_store_purchases.refund_event_at IS NULL OR excluded.refund_event_at>=reader_store_purchases.refund_event_at)
 THEN excluded.revoked_at ELSE reader_store_purchases.revoked_at END,
 refund_event_at=greatest(excluded.refund_event_at,reader_store_purchases.refund_event_at),
 signed_at=greatest(excluded.signed_at,reader_store_purchases.signed_at)
 WHERE (reader_store_purchases.reader_id,reader_store_purchases.bundle_id,reader_store_purchases.environment,reader_store_purchases.product_id)
 =(excluded.reader_id,excluded.bundle_id,excluded.environment,excluded.product_id)
