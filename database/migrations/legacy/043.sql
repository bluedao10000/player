CREATE TABLE episode_inventory_snapshots (
    checked_at timestamptz PRIMARY KEY,
    works jsonb NOT NULL CHECK (jsonb_typeof(works)='array' AND jsonb_array_length(works)>0)
);
