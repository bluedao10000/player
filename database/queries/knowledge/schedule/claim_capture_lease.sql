WITH next AS (SELECT p.* FROM episode_plans p JOIN research_topics t ON t.id=p.topic
    LEFT JOIN episode_policies pol ON pol.world_slug=p.world_slug LEFT JOIN scope_heads h ON h.topic=p.topic
    LEFT JOIN episode_source_captures c ON c.topic=p.topic
    WHERE t.enabled AND h.published IS NULL AND (c.topic IS NULL OR c.spec<>p.spec OR c.captured_at<now()-interval '12 hours')
    AND NOT EXISTS (SELECT FROM research_runs WHERE topic=p.topic AND stage='ready' AND policy=t.policy)
    AND NOT EXISTS(SELECT FROM episode_capture_attempts a WHERE a.topic=p.topic AND (a.finished_at IS NULL OR a.finished_at>now()-interval '15 minutes'))
    ORDER BY pol.priority DESC NULLS LAST,p.number,p.world_slug LIMIT 1), lease AS (
    INSERT INTO episode_capture_attempts(topic) SELECT topic FROM next RETURNING id)
    SELECT jsonb_build_object('lease',lease.id,'world',next.world_slug,'spec',next.spec) FROM next,lease
