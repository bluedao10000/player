SELECT p.topic FROM episode_plans p JOIN research_topics t ON t.id=p.topic AND t.enabled LEFT JOIN scope_heads h ON h.topic=p.topic WHERE h.published IS NULL ORDER BY p.number
