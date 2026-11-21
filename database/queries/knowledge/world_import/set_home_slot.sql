INSERT INTO home_world_slots VALUES($1,$2) ON CONFLICT(work_id) DO UPDATE SET position=excluded.position
