SELECT p.world_slug,p.number FROM episode_plans p JOIN research_topics t ON t.id=p.topic
		LEFT JOIN scope_heads h ON h.topic=p.topic LEFT JOIN episode_source_captures c ON c.topic=p.topic
		WHERE t.enabled AND h.published IS NULL AND (c.topic IS NULL OR c.spec<>p.spec OR c.captured_at<now()-interval '12 hours')
		ORDER BY p.world_slug,p.number
