UPDATE research_topics t SET enabled=false FROM episode_plans p WHERE t.id=p.topic AND p.world_slug=$1
    AND NOT EXISTS(SELECT FROM scope_heads WHERE topic=p.topic AND published IS NOT NULL)
    AND NOT EXISTS(SELECT FROM episode_admissions WHERE topic=p.topic)
    AND NOT EXISTS(SELECT FROM research_runs WHERE topic=p.topic)
