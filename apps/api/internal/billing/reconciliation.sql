-- Day is the first observation of a supplier receipt, in UTC. Cost rates use the
-- returned model snapshot if present. A missing rate leaves the amount unknown.
WITH receipts AS (
 SELECT * FROM billing_daily_costs($1,$2)
), local_cost AS (
 SELECT endpoint,provider_account,model,count(*) receipts,count(*)-count(cost_micros) pending, count(*) FILTER(WHERE multiple_subjects) attribution_conflicts,
 sum(cost_micros) known_cost_micros FROM receipts GROUP BY 1,2,3
), sales AS (
 SELECT v.endpoint,v.provider_account,v.model,
 sum(CASE WHEN o.status='refunded' THEN 0 ELSE o.charge_micros END) FILTER (WHERE r.currency=$2) revenue,
 count(*) FILTER (WHERE r.currency<>$2 AND o.status<>'refunded') other_currency
 FROM billing_orders o JOIN billing_rates r ON r.id=o.rate_id
 JOIN model_usage_receipts original ON original.id=o.receipt_id
 JOIN receipts v ON (v.endpoint,v.provider_account,v.receipt_key)=(original.endpoint,original.provider_account,original.receipt_key)
 GROUP BY 1,2,3
), summary AS (
 SELECT endpoint,provider_account,model,coalesce(l.receipts,0) receipts,coalesce(l.pending,0) pending, coalesce(l.attribution_conflicts,0) attribution_conflicts,
 l.known_cost_micros,s.reference,s.replaces,s.amount_micros supplier_micros,coalesce(t.revenue,0) net_revenue_micros,
 CASE WHEN l.receipts IS NULL THEN 'supplier_only' WHEN l.pending>0 OR l.attribution_conflicts>0 THEN 'pending'
 WHEN s.reference IS NULL THEN 'local_only' WHEN l.known_cost_micros=s.amount_micros THEN 'matched' ELSE 'difference' END status,
 CASE WHEN l.pending=0 THEN s.amount_micros-l.known_cost_micros END difference_micros,
 CASE WHEN l.pending=0 AND l.attribution_conflicts=0 AND l.known_cost_micros=s.amount_micros AND coalesce(t.other_currency,0)=0
 THEN coalesce(t.revenue,0)-s.amount_micros END margin_micros
 FROM local_cost l FULL JOIN (SELECT DISTINCT ON (endpoint,provider_account,model) * FROM billing_statements WHERE day=$1 AND currency=$2 ORDER BY endpoint,provider_account,model,id DESC) s
 USING(endpoint,provider_account,model)
 LEFT JOIN sales t USING(endpoint,provider_account,model)
)
SELECT jsonb_build_object('day',$1::date,'currency',$2::text,'groups',
 coalesce(jsonb_agg(to_jsonb(summary) ORDER BY endpoint,provider_account,model),'[]')) FROM summary
