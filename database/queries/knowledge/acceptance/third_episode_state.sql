SELECT EXISTS(SELECT FROM published_episode_scopes WHERE v=$1 AND first_episode=1 AND last_episode=2),
    coalesce(r.stage,'queued'),coalesce(r.error,'') FROM research_topics t LEFT JOIN LATERAL
    (SELECT stage,error FROM research_runs WHERE topic=t.id AND policy=t.policy AND v=$1 ORDER BY attempt DESC LIMIT 1) r ON true WHERE t.id=$2
