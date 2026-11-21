ALTER TABLE reader_wechat DROP CONSTRAINT reader_wechat_pkey;
ALTER TABLE reader_wechat ADD PRIMARY KEY (reader_id,app_id);
CREATE TABLE reader_wechat_union (
    subject text PRIMARY KEY,
    reader_id uuid NOT NULL UNIQUE REFERENCES reader_identities ON DELETE CASCADE
);
CREATE TABLE reader_apple (
    subject text PRIMARY KEY,
    reader_id uuid NOT NULL UNIQUE REFERENCES reader_identities ON DELETE CASCADE
);
CREATE TABLE reader_native_challenges (
    nonce text PRIMARY KEY, expires_at timestamptz NOT NULL DEFAULT now()+interval '5 minutes'
);
CREATE INDEX reader_native_challenge_expiry ON reader_native_challenges(expires_at);

CREATE TABLE reader_store_purchases (
    transaction_id text PRIMARY KEY,
    reader_id uuid NOT NULL REFERENCES reader_identities,
    environment text NOT NULL,
    bundle_id text NOT NULL, product_id text NOT NULL,
    plan text NOT NULL REFERENCES reader_plans CHECK(plan <> 'free'),
    credits integer NOT NULL CHECK(credits>0),
    purchased_at timestamptz NOT NULL,
    starts_at timestamptz NOT NULL, ends_at timestamptz NOT NULL CHECK(ends_at>starts_at),
    revoked_at timestamptz, signed_at timestamptz NOT NULL,
    currency text, price_milli bigint
);
CREATE INDEX reader_store_periods ON reader_store_purchases(reader_id,ends_at);
CREATE VIEW reader_paid_periods AS
 SELECT reader_id,plan,credits,starts_at,ends_at FROM reader_plan_orders WHERE paid_at IS NOT NULL AND refunded_at IS NULL
 UNION ALL SELECT reader_id,plan,credits,starts_at,ends_at FROM reader_store_purchases WHERE revoked_at IS NULL;
CREATE OR REPLACE VIEW reader_memberships AS
 SELECT r.id reader_id,p.id plan,p.shelf_limit,
    CASE WHEN o.plan IS NOT NULL THEN o.ends_at WHEN p.id<>'free' THEN r.plan_until END valid_until,
    coalesce(o.starts_at,date_trunc('month',now(),'Asia/Shanghai')) period_start,
    coalesce(o.ends_at,((date_trunc('month',now() AT TIME ZONE 'Asia/Shanghai')+interval '1 month') AT TIME ZONE 'Asia/Shanghai')) period_end,
    coalesce(o.credits,p.credits) credits
 FROM reader_identities r
 LEFT JOIN LATERAL (SELECT * FROM reader_paid_periods WHERE reader_id=r.id AND starts_at<=now() AND ends_at>now()
    ORDER BY starts_at DESC LIMIT 1) o ON true
 JOIN reader_plans p ON p.id=coalesce(o.plan,CASE WHEN r.plan_until<=now() THEN 'free' ELSE r.plan END);
