INSERT INTO knowledge_identity_pins SELECT unnest($1::bigint[]),$2::uuid ON CONFLICT(record_id) DO UPDATE SET work_id=excluded.work_id
