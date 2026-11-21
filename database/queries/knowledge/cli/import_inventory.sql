INSERT INTO episode_inventory_snapshots(checked_at,works) VALUES ($1::text::timestamptz,$2::jsonb) ON CONFLICT DO NOTHING
