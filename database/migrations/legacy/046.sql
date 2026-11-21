CREATE TABLE reader_release_attempts (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), topic text NOT NULL,
 acceptance bigint NOT NULL, started_at timestamptz NOT NULL DEFAULT now(),
 finished_at timestamptz, target_id bigint, href text, error text,
 FOREIGN KEY (topic,acceptance) REFERENCES scope_acceptances(topic,id),
 CHECK (finished_at IS NULL OR (target_id IS NOT NULL) <> (error IS NOT NULL))
);
CREATE INDEX reader_release_history ON reader_release_attempts(topic,acceptance,started_at DESC);
CREATE UNIQUE INDEX reader_release_once ON reader_release_attempts(topic,acceptance) WHERE target_id IS NOT NULL;
