INSERT INTO episode_art_jobs(world_slug,kind,slug,spec) VALUES($1,$2,$3,$4) ON CONFLICT(world_slug,kind,slug) DO UPDATE SET slug=excluded.slug RETURNING spec=$4::jsonb
