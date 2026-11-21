SELECT jsonb_build_object(
          'observed_at',(SELECT observed_at FROM backup_observer WHERE singleton),
          'latest_verified',(SELECT to_jsonb(r) FROM backup_runs r WHERE status='success' AND verified_tables IS NOT NULL ORDER BY finished_at DESC LIMIT 1),
          'runs',(SELECT coalesce(jsonb_agg(to_jsonb(r) ORDER BY started_at DESC,job_uid),'[]')
                  FROM (SELECT limited.*, (SELECT coalesce(jsonb_agg(jsonb_build_object('at',a.finished_at,'reason',a.reason) ORDER BY a.finished_at),'[]')
                          FROM backup_failed_attempts a WHERE a.job_uid=limited.job_uid) AS attempts
                        FROM (SELECT * FROM backup_runs ORDER BY started_at DESC,job_uid LIMIT 30) limited) r))
