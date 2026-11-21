SELECT coalesce(jsonb_agg(to_jsonb(q) ORDER BY admitted_at DESC,started_at DESC NULLS LAST),'[]')
		FROM (SELECT p.topic,p.world_slug,p.number,p.spec->>'title' AS title,u.admitted_at,u.measured_tokens,
		  r.started_at,r.finished_at,r.error,r.href,r.target_id,
		  CASE WHEN r.target_id IS NOT NULL THEN '线上已验证' WHEN r.error IS NOT NULL AND EXISTS
		    (SELECT FROM reader_release_attempts later WHERE later.topic=r.topic AND later.acceptance=r.acceptance
		     AND later.started_at>r.started_at AND later.target_id IS NOT NULL) THEN '本次失败 · 后续已发布'
		  WHEN r.error IS NOT NULL THEN '待重试'
		  WHEN r.id IS NOT NULL THEN '发布中' WHEN s.id IS NOT NULL THEN '待同步' ELSE '等待五维验收' END AS status
		  FROM episode_plans p JOIN episode_admission_usage u USING(topic)
		  LEFT JOIN visible_reader_scopes s USING(topic) LEFT JOIN reader_release_attempts r ON r.topic=p.topic
		  WHERE p.spec @> '{"publishToReader":true}' ORDER BY u.admitted_at DESC,r.started_at DESC LIMIT 100)q
