ALTER TABLE billing_orders ADD COLUMN automatic boolean NOT NULL DEFAULT false;
CREATE INDEX billing_queued_previews ON billing_orders(seq) WHERE automatic AND status='reserved';

CREATE TABLE billing_reader_sessions (
 token_hash bytea PRIMARY KEY CHECK (octet_length(token_hash)=32),
 account_id uuid NOT NULL REFERENCES billing_accounts,
 expires_at timestamptz NOT NULL DEFAULT now()+interval '12 hours'
);
CREATE INDEX billing_reader_session_expiry ON billing_reader_sessions(expires_at);

CREATE TABLE billing_preview_workers (
 endpoint text NOT NULL, provider_account text NOT NULL, model text NOT NULL,
 available_until timestamptz NOT NULL,
 PRIMARY KEY(endpoint,provider_account,model)
);
