CREATE TABLE reader_identities (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email text NOT NULL UNIQUE,
    created_at timestamptz NOT NULL DEFAULT now(),
    import_choice text CHECK (import_choice IN ('merge','skip'))
);
CREATE TABLE reader_email_codes (
    email text PRIMARY KEY,
    digest bytea NOT NULL,
    issued_at timestamptz NOT NULL DEFAULT now(),
    attempts int NOT NULL DEFAULT 0
);
CREATE INDEX reader_codes_issued ON reader_email_codes(issued_at);
CREATE TABLE reader_sessions (
    digest bytea PRIMARY KEY,
    reader_id uuid NOT NULL REFERENCES reader_identities ON DELETE CASCADE,
    expires_at timestamptz NOT NULL
);
CREATE INDEX reader_session_expiry ON reader_sessions(expires_at);
CREATE TABLE reader_bookmarks (
    reader_id uuid NOT NULL REFERENCES reader_identities ON DELETE CASCADE,
    href text NOT NULL,
    title text NOT NULL,
    work text NOT NULL,
    recorded_at bigint NOT NULL,
    PRIMARY KEY(reader_id, href)
);
CREATE TABLE reader_positions (
    reader_id uuid NOT NULL REFERENCES reader_identities ON DELETE CASCADE,
    href text NOT NULL,
    title text NOT NULL,
    work text NOT NULL,
    percent int NOT NULL CHECK(percent BETWEEN 0 AND 100),
    anchor text NOT NULL,
    excerpt text NOT NULL,
    recorded_at bigint NOT NULL,
    PRIMARY KEY(reader_id, href)
);
