CREATE TABLE backup_runs (
    job_uid uuid PRIMARY KEY,
    job_name text NOT NULL,
    started_at timestamptz NOT NULL,
    finished_at timestamptz,
    status text NOT NULL CHECK (status IN ('running','success','failed')),
    archive_bytes bigint CHECK (archive_bytes > 0),
    verified_tables integer CHECK (verified_tables > 0),
    failure_reason text,
    CHECK ((archive_bytes IS NULL) = (verified_tables IS NULL)),
    CHECK (archive_bytes IS NULL OR status = 'success'),
    CHECK ((finished_at IS NULL) = (status = 'running')),
    CHECK ((failure_reason IS NOT NULL) = (status = 'failed'))
);
CREATE INDEX backup_runs_started ON backup_runs(started_at DESC);
CREATE TABLE backup_observer (
    singleton boolean PRIMARY KEY DEFAULT true CHECK (singleton),
    observed_at timestamptz NOT NULL
);
