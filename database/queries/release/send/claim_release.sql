WITH due AS (SELECT s.topic,s.id FROM visible_reader_scopes s JOIN episode_admissions a USING(topic)
 JOIN episode_plans p USING(topic) JOIN episode_policies pol ON pol.world_slug=p.world_slug
 WHERE p.spec @> '{"publishToReader":true}' AND NOT EXISTS(SELECT FROM reader_release_attempts r WHERE r.topic=s.topic
 AND r.acceptance=s.id AND (r.target_id IS NOT NULL OR r.finished_at>now()-interval '5 minutes'))
 ORDER BY pol.priority DESC,s.first_episode,s.topic LIMIT 1)
 INSERT INTO reader_release_attempts(topic,acceptance) SELECT * FROM due RETURNING id::text,topic,acceptance
