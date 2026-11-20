CREATE TABLE reader_plans (
    id text PRIMARY KEY CHECK (id IN ('free','plus','pro')),
    shelf_limit integer NOT NULL CHECK (shelf_limit>0)
);
INSERT INTO reader_plans VALUES ('free',100),('plus',500),('pro',2000);
ALTER TABLE reader_identities
    ADD COLUMN plan text NOT NULL DEFAULT 'free' REFERENCES reader_plans,
    ADD COLUMN plan_until timestamptz;
CREATE TABLE reader_plan_changes (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    reader_id uuid NOT NULL REFERENCES reader_identities ON DELETE CASCADE,
    plan text NOT NULL REFERENCES reader_plans,
    valid_until timestamptz,
    operator text NOT NULL,
    changed_at timestamptz NOT NULL DEFAULT now()
);
CREATE VIEW reader_memberships AS
    SELECT r.id AS reader_id,p.id AS plan,p.shelf_limit,
           CASE WHEN p.id<>'free' THEN r.plan_until END AS valid_until
    FROM reader_identities r
    JOIN reader_plans p ON p.id=CASE WHEN r.plan_until<=now() THEN 'free' ELSE r.plan END;
