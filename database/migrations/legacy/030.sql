-- Monetary amounts are integer millionths of one currency unit. No live prices are seeded.
CREATE TABLE billing_accounts (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 customer_ref text NOT NULL UNIQUE CHECK (length(customer_ref) BETWEEN 1 AND 150),
 token_hash bytea NOT NULL UNIQUE CHECK (octet_length(token_hash)=32),
 currency text NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
 created_by text NOT NULL, created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE billing_rates (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 kind text NOT NULL CHECK (kind IN ('retail','cost')),
 endpoint text NOT NULL CHECK (length(endpoint) BETWEEN 1 AND 512),
 provider_account text NOT NULL CHECK (length(provider_account) BETWEEN 1 AND 128),
 model text NOT NULL CHECK (length(model) BETWEEN 1 AND 200),
 currency text NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
 effective_at timestamptz NOT NULL,
 unit text NOT NULL CHECK (unit IN ('total_tokens','split_tokens','image')),
 unit_micros bigint NOT NULL DEFAULT 0 CHECK (unit_micros>=0),
 input_micros bigint NOT NULL DEFAULT 0 CHECK (input_micros>=0),
 output_micros bigint NOT NULL DEFAULT 0 CHECK (output_micros>=0),
 cached_micros bigint CHECK (cached_micros>=0),
 cap_micros bigint NOT NULL DEFAULT 0,
 created_by text NOT NULL,
 CHECK ((kind='cost' AND cap_micros=0) OR (kind='retail' AND cap_micros>0)),
 CHECK ((unit='split_tokens' AND unit_micros=0) OR (unit<>'split_tokens' AND input_micros=0 AND output_micros=0 AND cached_micros IS NULL)),
 UNIQUE(kind,currency,endpoint,provider_account,model,effective_at)
);
-- The first retail product is a single private art preview per order.
CREATE UNIQUE INDEX billing_retail_version ON billing_rates(currency,effective_at) WHERE kind='retail';

CREATE TABLE billing_orders (
 seq bigint GENERATED ALWAYS AS IDENTITY UNIQUE,
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), account_id uuid NOT NULL REFERENCES billing_accounts,
 client_key text NOT NULL CHECK(length(client_key) BETWEEN 1 AND 128),
 rate_id uuid NOT NULL REFERENCES billing_rates, request jsonb NOT NULL,
 status text NOT NULL DEFAULT 'reserved' CHECK(status IN ('reserved','running','delivered','settled','void','refunded')),
 receipt_id uuid UNIQUE REFERENCES model_calls, charge_micros bigint CHECK(charge_micros>=0),
 units jsonb, delivery_sha256 text CHECK(delivery_sha256 ~ '^[0-9a-f]{64}$'),
 created_at timestamptz NOT NULL DEFAULT now(), finished_at timestamptz, reason text, next_check_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(account_id,client_key)
);
CREATE INDEX billing_open_orders ON billing_orders(account_id) WHERE status IN ('reserved','running','delivered');
CREATE INDEX billing_recovery_due ON billing_orders(next_check_at) WHERE status IN ('running','delivered');
CREATE TABLE billing_entries (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY, account_id uuid NOT NULL REFERENCES billing_accounts,
 kind text NOT NULL CHECK(kind IN ('credit','charge','refund')), amount_micros bigint NOT NULL,
 reference text NOT NULL CHECK(length(reference) BETWEEN 1 AND 128),
 order_id uuid REFERENCES billing_orders, actor text NOT NULL, at timestamptz NOT NULL DEFAULT now(),
 CHECK ((kind='credit' AND order_id IS NULL AND amount_micros>0) OR
        (kind='charge' AND order_id IS NOT NULL AND amount_micros<=0) OR
        (kind='refund' AND order_id IS NOT NULL AND amount_micros>=0)),
 UNIQUE(account_id,kind,reference), UNIQUE(order_id,kind)
);
CREATE UNIQUE INDEX billing_credit_source ON billing_entries(reference) WHERE kind='credit';
CREATE TABLE billing_statements (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY, recorded_at timestamptz NOT NULL DEFAULT now(), replaces text NOT NULL DEFAULT '',
 day date NOT NULL, endpoint text NOT NULL, provider_account text NOT NULL, model text NOT NULL,
 currency text NOT NULL CHECK(currency ~ '^[A-Z]{3}$'), amount_micros bigint NOT NULL CHECK(amount_micros>=0),
 reference text NOT NULL CHECK(length(reference) BETWEEN 1 AND 150), created_by text NOT NULL,
 UNIQUE(day,endpoint,provider_account,model,currency,reference)
);
CREATE FUNCTION billing_append_only() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 RAISE EXCEPTION 'financial history is append-only';
END $$;
CREATE TRIGGER billing_prices_immutable BEFORE UPDATE OR DELETE ON billing_rates FOR EACH ROW EXECUTE FUNCTION billing_append_only();
CREATE TRIGGER billing_entries_immutable BEFORE UPDATE OR DELETE ON billing_entries FOR EACH ROW EXECUTE FUNCTION billing_append_only();
CREATE TRIGGER billing_statements_immutable BEFORE UPDATE OR DELETE ON billing_statements FOR EACH ROW EXECUTE FUNCTION billing_append_only();

CREATE VIEW billing_balances AS
SELECT a.id,a.customer_ref,a.currency,coalesce(l.balance,0) balance_micros,
 coalesce(h.held,0) held_micros,coalesce(l.balance,0)-coalesce(h.held,0) available_micros
FROM billing_accounts a
LEFT JOIN LATERAL (SELECT sum(amount_micros) balance FROM billing_entries WHERE account_id=a.id) l ON true
LEFT JOIN LATERAL (SELECT sum(r.cap_micros) held FROM billing_orders o JOIN billing_rates r ON r.id=o.rate_id
 WHERE o.account_id=a.id AND o.status IN ('reserved','running','delivered')) h ON true;

-- A missing partition only matters when that partition has a different price.
CREATE FUNCTION billing_amount(r billing_rates, state text, t bigint, i bigint, o bigint, images integer, cached numeric)
RETURNS numeric LANGUAGE sql IMMUTABLE AS $$
 SELECT CASE
 WHEN state IN ('inconsistent','unresolved') OR t<i OR t<o THEN NULL
 WHEN r.unit='image' THEN images::numeric*r.unit_micros
 WHEN state<>'measured' THEN NULL
 WHEN r.unit='total_tokens' THEN div(t::numeric*r.unit_micros+500000,1000000)
 WHEN r.unit='split_tokens' AND i IS NOT NULL AND o IS NOT NULL THEN
  CASE WHEN r.cached_micros IS NULL OR r.cached_micros=r.input_micros THEN
    div(i::numeric*r.input_micros+o::numeric*r.output_micros+500000,1000000)
  WHEN cached BETWEEN 0 AND i THEN
    div((i-cached)*r.input_micros+cached*r.cached_micros+o::numeric*r.output_micros+500000,1000000)
  END
 END
$$;
CREATE FUNCTION billing_cached_tokens(u jsonb) RETURNS numeric LANGUAGE sql IMMUTABLE AS $$
 SELECT CASE WHEN v ~ '^[0-9]+$' THEN v::numeric END FROM (SELECT coalesce(u#>>'{input_tokens_details,cached_tokens}',u#>>'{prompt_tokens_details,cached_tokens}',u->>'prompt_cache_hit_tokens') v) counts
$$;
CREATE VIEW billing_order_details AS
SELECT o.*, r.currency,r.cap_micros FROM billing_orders o JOIN billing_rates r ON r.id=o.rate_id;

-- Only a deliberately dispatched order may own a new billable transport attempt.
CREATE FUNCTION billing_identify_order() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.subject LIKE 'order:%' THEN
  IF NOT EXISTS(SELECT 1 FROM billing_orders o JOIN billing_rates r ON r.id=o.rate_id
     WHERE o.id::text=substring(NEW.subject FROM 7) AND NEW.purpose='image' AND NEW.operation='images/generations'
     AND NEW.image_size=o.request->>'size'
     AND r.endpoint=NEW.endpoint AND r.provider_account=NEW.provider_account AND r.model=NEW.requested_model)
  THEN RAISE EXCEPTION 'invalid billing attribution'; END IF;
  NEW.cost_owner := 'customer';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER billing_call_owner BEFORE INSERT ON model_calls FOR EACH ROW EXECUTE FUNCTION billing_identify_order();

-- Prefer complete supplier evidence without counting a replay twice. A cache breakdown
-- matters financially even when two observations agree on their total tokens.
CREATE VIEW billing_receipts AS
 WITH merged AS (
  SELECT endpoint,provider_account,receipt_key,max(id::text) FILTER (WHERE receipt_rank=1)::uuid call_id,
    min(receipt_started_at) started_at,coalesce(max(resolved_model),max(requested_model)) model,
    min(subject) subject, min(subject)<>max(subject) multiple_subjects,
    bool_or(outcome='ok') successful,bool_or(outcome IN ('started','unknown')) unfinished,
    bool_or(meter_state='inconsistent') OR
    min(billing_cached_tokens(usage)) IS DISTINCT FROM max(billing_cached_tokens(usage)) AS conflicting,
    max(total_tokens) total_tokens,max(input_tokens) input_tokens,max(output_tokens) output_tokens,
    max(images) images,max(billing_cached_tokens(usage)) cached
  FROM model_usage_receipts GROUP BY endpoint,provider_account,receipt_key
 )
 SELECT m.*,CASE WHEN conflicting OR cached>input_tokens OR total_tokens<input_tokens OR total_tokens<output_tokens
    OR total_tokens::numeric<>input_tokens::numeric+output_tokens THEN 'inconsistent'
   WHEN total_tokens IS NOT NULL THEN 'measured'
   WHEN input_tokens IS NOT NULL OR output_tokens IS NOT NULL THEN 'partial'
   WHEN unfinished THEN 'unresolved' ELSE 'unreported' END meter_state FROM merged m;

CREATE FUNCTION billing_daily_costs(d date,cur text)
RETURNS TABLE(call_id uuid,endpoint text,provider_account text,receipt_key text,model text,subject text,meter_state text,rate_id uuid,cost_micros numeric,multiple_subjects boolean)
LANGUAGE sql STABLE AS $$
 SELECT v.call_id,v.endpoint,v.provider_account,v.receipt_key,v.model,v.subject,v.meter_state,r.id,
  billing_amount(r,v.meter_state,v.total_tokens,v.input_tokens,v.output_tokens,v.images,v.cached),v.multiple_subjects
 FROM billing_receipts v LEFT JOIN LATERAL (
 SELECT p.* FROM billing_rates p WHERE p.kind='cost' AND p.endpoint=v.endpoint AND p.provider_account=v.provider_account
  AND p.model=v.model AND p.currency=cur AND p.effective_at<=v.started_at ORDER BY effective_at DESC LIMIT 1
 ) r ON true
 WHERE v.started_at >= (d::timestamp AT TIME ZONE 'UTC') AND v.started_at < ((d+1)::timestamp AT TIME ZONE 'UTC')
$$;
