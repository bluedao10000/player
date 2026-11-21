WITH next AS (SELECT topic,reserve_tokens FROM episode_policy_queue WHERE status='待准入'
    ORDER BY priority DESC,number,world_slug LIMIT 1), admitted AS (
    INSERT INTO episode_admissions(topic,reserve_tokens) SELECT * FROM next ON CONFLICT DO NOTHING RETURNING topic)
    UPDATE research_topics SET enabled=true FROM admitted WHERE id=admitted.topic
