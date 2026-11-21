CREATE TABLE backup_failed_attempts (
    pod_uid uuid PRIMARY KEY,
    job_uid uuid NOT NULL REFERENCES backup_runs(job_uid),
    finished_at timestamptz NOT NULL,
    reason text NOT NULL
);
CREATE INDEX backup_attempts_by_job ON backup_failed_attempts(job_uid, finished_at);
