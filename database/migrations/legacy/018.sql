-- slot remains the global FK for audit history. position is local to the batch.
ALTER TABLE media_candidates DROP CONSTRAINT media_candidates_slot_check;
ALTER TABLE media_candidates DROP CONSTRAINT media_candidates_stage_check;
ALTER TABLE media_candidates DROP CONSTRAINT media_candidates_scene_variant_key;
ALTER TABLE media_candidates ADD COLUMN batch text NOT NULL DEFAULT 'mio',
 ADD COLUMN position integer, ADD COLUMN reference_sha text NOT NULL DEFAULT '';
UPDATE media_candidates SET position=slot;
ALTER TABLE media_candidates ALTER COLUMN position SET NOT NULL,
 ADD UNIQUE(batch,position), ADD UNIQUE(batch,scene,variant),
 ADD CHECK(position BETWEEN 1 AND 9),
 ADD CHECK(stage IN ('reference','queued','generate','review','recrop','recheck','passed','held','failed'));
INSERT INTO media_candidates(slot,batch,position,scene,variant,stage)
 SELECT 9+i,'bebop',i,(ARRAY['药瓶赴约','宠物店开箱','街头遛狗'])[(i-1)/3+1],(i-1)%3+1,'reference'
 FROM generate_series(1,9)i;
