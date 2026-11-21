SELECT count(*) FROM reader_plan_orders WHERE reader_id=$1 AND ends_at=starts_at+interval '720 hours'
