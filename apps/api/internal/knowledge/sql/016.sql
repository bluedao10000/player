ALTER TABLE media_candidates ADD COLUMN attempt integer NOT NULL DEFAULT 1 CHECK(attempt IN (1,2));
ALTER TABLE media_steps ADD COLUMN attempt integer NOT NULL DEFAULT 1 CHECK(attempt IN (1,2));
ALTER TABLE media_steps DROP CONSTRAINT media_steps_pkey;
ALTER TABLE media_steps ADD PRIMARY KEY(slot,round,step,attempt);
-- Recover first-deployment write failures after the volume owner is initialized.
UPDATE media_candidates SET stage='generate',attempt=2,reason='',updated_at=now()
 WHERE stage='failed' AND round=1 AND reason LIKE 'EACCES:%/media/%';
