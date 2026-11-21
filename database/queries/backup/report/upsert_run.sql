INSERT INTO backup_runs(job_uid,job_name,started_at,finished_at,status,archive_bytes,verified_tables,failure_reason,trigger_kind)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)
 ON CONFLICT(job_uid) DO UPDATE SET finished_at=excluded.finished_at,status=excluded.status,
 archive_bytes=coalesce(excluded.archive_bytes,backup_runs.archive_bytes),
 verified_tables=coalesce(excluded.verified_tables,backup_runs.verified_tables),
 failure_reason=coalesce(backup_runs.failure_reason,excluded.failure_reason),
 trigger_kind=CASE WHEN excluded.trigger_kind='unknown' THEN backup_runs.trigger_kind ELSE excluded.trigger_kind END
 WHERE backup_runs.status='running' OR
 (backup_runs.status=excluded.status AND backup_runs.trigger_kind='unknown' AND excluded.trigger_kind<>'unknown') OR (backup_runs.status='success' AND backup_runs.archive_bytes IS NULL AND excluded.archive_bytes IS NOT NULL)
