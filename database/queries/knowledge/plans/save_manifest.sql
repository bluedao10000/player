INSERT INTO episode_plan_manifests VALUES($1,$2,now()) ON CONFLICT(world_slug) DO UPDATE SET checksum=excluded.checksum,loaded_at=now()
