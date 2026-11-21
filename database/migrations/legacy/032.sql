CREATE TABLE billing_alipay_topups (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 account_id uuid NOT NULL REFERENCES billing_accounts, client_key uuid NOT NULL,
 amount_fen integer NOT NULL CHECK (amount_fen BETWEEN 1 AND 1000000),
 app_id text NOT NULL, seller_id text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 expires_at timestamptz NOT NULL DEFAULT now()+interval '30 minutes',
 trade_no text UNIQUE, paid_at timestamptz,
 closed boolean NOT NULL DEFAULT false,
 entry_id bigint UNIQUE REFERENCES billing_entries,
 UNIQUE(account_id,client_key),
 CHECK ((paid_at IS NULL)=(entry_id IS NULL) AND (trade_no IS NULL)=(entry_id IS NULL))
);
CREATE INDEX billing_topups_reader ON billing_alipay_topups(account_id,created_at DESC);
