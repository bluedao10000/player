CREATE TABLE episode_source_captures (
 topic text PRIMARY KEY REFERENCES episode_plans(topic),
 spec jsonb NOT NULL,
 documents jsonb NOT NULL CHECK(jsonb_typeof(documents)='array'),
 captured_at timestamptz NOT NULL
);
