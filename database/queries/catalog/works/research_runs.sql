SELECT coalesce(jsonb_agg(to_jsonb(x) ORDER BY x.started_at DESC),'[]') FROM (
			SELECT r.id,r.started_at,r.finished_at,r.stage,r.attempt,r.policy,r.policy=t.policy AS current_policy,r.model,r.error,w.name||' · '||w.edition AS edition_title,t.world_slug,t.first_episode,t.last_episode,r.v AS version_id,
			(SELECT jsonb_agg(d-'text'-'sentences' ORDER BY d->>'id') FROM jsonb_array_elements(r.documents) d) AS documents,
			(SELECT jsonb_agg(jsonb_build_object('at',e.at,'message',e.message) ORDER BY e.id) FROM research_events e WHERE e.run_id=r.id) AS events,
			jsonb_array_length(r.claims) AS paragraphs
			FROM research_runs r JOIN research_topics t ON t.id=r.topic JOIN work_worlds w ON w.slug=t.world_slug ORDER BY r.started_at DESC LIMIT $1)x
