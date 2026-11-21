SELECT p.spec,'cfg-'||a.id,coalesce(r.stage,'queued'),coalesce(r.error,''),EXISTS(SELECT FROM scope_heads h WHERE h.topic=p.topic AND h.published IS NOT NULL)
 FROM episode_plans p JOIN episode_art_jobs a ON a.world_slug=p.world_slug AND a.kind='scene' AND a.slug=p.spec->>'sceneSlug'
 LEFT JOIN LATERAL(SELECT stage,error FROM research_runs WHERE topic=p.topic ORDER BY policy DESC,attempt DESC LIMIT 1) r ON true WHERE p.topic=$1
