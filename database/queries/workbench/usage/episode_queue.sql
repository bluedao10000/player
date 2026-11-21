SELECT coalesce(jsonb_agg(to_jsonb(q)||jsonb_build_object('capture_failures',
  (SELECT jsonb_agg(jsonb_build_object('at',finished_at,'reason',error) ORDER BY finished_at)
   FROM episode_capture_attempts a WHERE a.topic=q.topic AND a.error IS NOT NULL)) ORDER BY priority DESC,number,world_slug),'[]') FROM episode_policy_queue q
