ALTER TABLE reader_plans ADD COLUMN price_fen integer NOT NULL DEFAULT 0 CHECK(price_fen>=0),
    ADD COLUMN credits integer NOT NULL DEFAULT 0 CHECK(credits>=0);
UPDATE reader_plans p SET price_fen=v.fen,credits=v.credits FROM
    (VALUES ('free',0,10),('plus',2000,125),('pro',9900,625)) AS v(id,fen,credits) WHERE p.id=v.id;

CREATE TABLE reader_plan_orders (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    reader_id uuid NOT NULL REFERENCES reader_identities,
    plan text NOT NULL REFERENCES reader_plans CHECK(plan<>'free'),
    amount_fen integer NOT NULL CHECK(amount_fen>0),
    credits integer NOT NULL CHECK(credits>0),
    app_id text NOT NULL,seller_id text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL DEFAULT now()+interval '15 minutes',
    trade_no text UNIQUE, closed boolean NOT NULL DEFAULT false,
    paid_at timestamptz, starts_at timestamptz, ends_at timestamptz,
    refunded_at timestamptz,
    CHECK ((paid_at IS NULL)=(trade_no IS NULL) AND (starts_at IS NULL)=(paid_at IS NULL)
      AND (ends_at IS NULL)=(paid_at IS NULL) AND ends_at>starts_at)
);
CREATE INDEX reader_plan_order_history ON reader_plan_orders(reader_id,created_at DESC);
CREATE TABLE reader_credit_usage (
    reader_id uuid NOT NULL REFERENCES reader_identities,
    operation_id uuid NOT NULL UNIQUE,
    period_start timestamptz NOT NULL,
    microcredits bigint NOT NULL CHECK(microcredits>0),
    recorded_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY(reader_id,operation_id)
);
CREATE INDEX reader_credit_period ON reader_credit_usage(reader_id,period_start);
CREATE TRIGGER reader_credit_usage_immutable BEFORE UPDATE OR DELETE ON reader_credit_usage
    FOR EACH ROW EXECUTE FUNCTION billing_append_only();

-- A paid period takes precedence over an operational grant. Both use the same reader ID.
CREATE OR REPLACE VIEW reader_memberships AS
 SELECT r.id reader_id,p.id plan,p.shelf_limit,
    CASE WHEN o.id IS NOT NULL THEN o.ends_at WHEN p.id<>'free' THEN r.plan_until END valid_until,
    coalesce(o.starts_at,date_trunc('month',now(),'Asia/Shanghai')) period_start,
    coalesce(o.ends_at,((date_trunc('month',now() AT TIME ZONE 'Asia/Shanghai')+interval '1 month') AT TIME ZONE 'Asia/Shanghai')) period_end,
    coalesce(o.credits,p.credits) credits
 FROM reader_identities r
 LEFT JOIN LATERAL (SELECT * FROM reader_plan_orders WHERE reader_id=r.id AND refunded_at IS NULL
    AND starts_at<=now() AND ends_at>now() ORDER BY starts_at DESC LIMIT 1) o ON true
 JOIN reader_plans p ON p.id=coalesce(o.plan,CASE WHEN r.plan_until<=now() THEN 'free' ELSE r.plan END);
