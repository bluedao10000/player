CREATE TABLE knowledge_quality_runs(
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 started_at timestamptz NOT NULL DEFAULT now(), finished_at timestamptz,
 status text NOT NULL CHECK(status IN ('running','success','failed')),
 report jsonb, error_message text
);
CREATE INDEX quality_runs_recent ON knowledge_quality_runs(started_at DESC);
