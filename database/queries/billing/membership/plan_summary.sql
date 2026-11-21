SELECT jsonb_build_object('enabled',$2::boolean,'periodStart',period_start,'periodEnd',period_end,
 'plan',plan,'credits',credits,'usedMicro',coalesce((SELECT sum(microcredits) FROM reader_credit_usage WHERE reader_id=m.reader_id AND period_start=m.period_start),0),
 'plans',(SELECT jsonb_agg(jsonb_build_object('id',id,'fen',price_fen,'credits',credits) ORDER BY price_fen) FROM reader_plans),
 'storeOrders',(SELECT coalesce(jsonb_agg(to_jsonb(x) ORDER BY x.purchased_at DESC),'[]') FROM
 (SELECT transaction_id AS id,plan,purchased_at,currency,price_milli,revoked_at IS NOT NULL AS refunded
 FROM reader_store_purchases WHERE reader_id=m.reader_id ORDER BY purchased_at DESC LIMIT 20)x),
 'orders',(SELECT coalesce(jsonb_agg(to_jsonb(x) ORDER BY x.created_at DESC),'[]') FROM
 (SELECT id,plan,amount_fen,created_at,starts_at,ends_at,CASE WHEN refunded_at IS NOT NULL THEN 'refunded' WHEN paid_at IS NOT NULL THEN 'paid'
 WHEN expires_at<=now() OR closed THEN 'closed' ELSE 'pending' END status FROM reader_plan_orders WHERE reader_id=m.reader_id ORDER BY created_at DESC LIMIT 10)x))
 FROM reader_memberships m WHERE reader_id=$1
