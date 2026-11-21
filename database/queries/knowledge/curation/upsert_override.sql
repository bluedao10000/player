INSERT INTO work_field_overrides VALUES($1,$2,$3) ON CONFLICT(work_id,field) DO UPDATE SET value=excluded.value
