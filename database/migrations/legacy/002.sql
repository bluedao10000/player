ALTER TABLE knowledge_sync_runs ADD COLUMN skip_reason text;
UPDATE knowledge_sync_runs SET skip_reason='version_unchanged' WHERE skipped;
