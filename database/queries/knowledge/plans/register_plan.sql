INSERT INTO episode_plans(topic,world_slug,number,spec) VALUES($1,$2,$3,$4) ON CONFLICT(topic) DO UPDATE SET topic=excluded.topic RETURNING spec=$4::jsonb
