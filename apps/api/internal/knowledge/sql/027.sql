CREATE TABLE episode_plan_manifests (
 world_slug text PRIMARY KEY REFERENCES work_worlds(slug), checksum text NOT NULL,
 loaded_at timestamptz NOT NULL DEFAULT now()
);
