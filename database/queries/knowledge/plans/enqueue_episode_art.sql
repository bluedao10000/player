INSERT INTO media_candidates(slot,batch,position,scene,variant,stage,art_job)
 SELECT nextval('episode_candidate_slots'),'cfg-'||a.id,i,a.spec->>'title',i,'reference',a.id
 FROM episode_plans p JOIN episode_art_jobs a ON a.world_slug=p.world_slug CROSS JOIN LATERAL generate_series(1,CASE a.kind WHEN 'scene' THEN 3 ELSE 1 END)i
 WHERE p.topic=$1 AND ((a.kind='scene' AND a.slug=p.spec->>'sceneSlug') OR (a.kind='portrait' AND p.spec->'people' @> jsonb_build_array(jsonb_build_object('slug',a.slug))))
 AND NOT EXISTS(SELECT FROM media_candidates c WHERE c.art_job=a.id AND position=i) ON CONFLICT(batch,position) DO NOTHING
