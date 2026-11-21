ALTER TABLE backup_runs ADD COLUMN trigger_kind text NOT NULL DEFAULT 'unknown'
    CHECK (trigger_kind IN ('manual','scheduled','unknown'));
