-- One new acceptance. IDs with local sequences are allocated at the destination.
WITH p AS (SELECT $1::jsonb j),
 t AS (INSERT INTO research_topics SELECT x.* FROM p,jsonb_populate_record(NULL::research_topics,j->'research_topics'->0)x RETURNING id),
 r AS (INSERT INTO research_runs SELECT x.* FROM p,t,jsonb_populate_record(NULL::research_runs,j->'research_runs'->0)x WHERE x.topic=t.id RETURNING id),
 e AS (INSERT INTO episode_extensions SELECT x.* FROM p,t,jsonb_populate_record(NULL::episode_extensions,j->'episode_extensions'->0)x WHERE x.topic=t.id),
 plan AS (INSERT INTO episode_plans(topic,world_slug,number,spec)
 SELECT x.topic,x.world_slug,x.number,x.spec FROM p,t,jsonb_populate_record(NULL::episode_plans,j->'episode_plans'->0)x WHERE x.topic=t.id),
 art AS (INSERT INTO episode_art_jobs(world_slug,kind,slug,spec)
 SELECT x.world_slug,x.kind,x.slug,x.spec FROM p,jsonb_populate_record(NULL::episode_art_jobs,j->'episode_art_jobs'->0)x RETURNING id),
 media AS (INSERT INTO media_candidates(slot,batch,position,scene,variant,stage,round,focus,reason,updated_at,attempt,reference_sha,character_slug,art_job)
 SELECT nextval('episode_candidate_slots'),'cfg-'||art.id,x.position,x.scene,x.variant,x.stage,x.round,x.focus,x.reason,x.updated_at,x.attempt,x.reference_sha,x.character_slug,art.id
 FROM p,art,jsonb_populate_record(NULL::media_candidates,j->'media_candidates'->0)x RETURNING slot),
 a AS (INSERT INTO scope_acceptances(topic,v,rule,fingerprint,checked_at,passed,checks,picks,edition_slug,scope_first,scope_last,research_run,reader_copy)
 SELECT x.topic,x.v,x.rule,x.fingerprint,x.checked_at,x.passed,x.checks,jsonb_set(x.picks,'{0,slot}',to_jsonb(media.slot)),
 x.edition_slug,x.scope_first,x.scope_last,r.id,x.reader_copy FROM p,r,media,jsonb_populate_record(NULL::scope_acceptances,j->'scope_acceptances'->0)x
 RETURNING topic,id)
INSERT INTO scope_heads(topic,latest,published) SELECT topic,id,id FROM a RETURNING published
