-- Existing email accounts keep their reader ID, sessions and reading history.
ALTER TABLE reader_identities ALTER COLUMN email DROP NOT NULL;
CREATE TABLE reader_wechat (
  reader_id uuid PRIMARY KEY REFERENCES reader_identities ON DELETE CASCADE,
  app_id text NOT NULL,
  open_id text NOT NULL,
  UNIQUE(app_id, open_id)
);
CREATE TABLE reader_wechat_flows (
  state text PRIMARY KEY,
  browser_hash bytea NOT NULL,
  bind_session bytea REFERENCES reader_sessions(digest) ON DELETE CASCADE,
  return_to text NOT NULL,
  expires_at timestamptz NOT NULL
);
CREATE INDEX reader_wechat_flows_expiry ON reader_wechat_flows(expires_at);
