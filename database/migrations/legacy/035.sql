-- Each operator-authorized resume gets a fresh Pi task identity. The scheduler
-- still makes at most one automatic retry per initial provider failure.
ALTER TABLE media_candidates DROP CONSTRAINT media_candidates_attempt_check,
  ADD CHECK (attempt >= 1);
ALTER TABLE media_steps DROP CONSTRAINT media_steps_attempt_check,
  ADD CHECK (attempt >= 1);
