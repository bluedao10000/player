INSERT INTO backup_failed_attempts(pod_uid,job_uid,finished_at,reason)
                VALUES($1,$2,$3,$4) ON CONFLICT(pod_uid) DO NOTHING
