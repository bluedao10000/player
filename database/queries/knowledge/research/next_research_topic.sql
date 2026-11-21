SELECT t.id,t.policy,w.version_id,coalesce(last.attempt,0),t.first_episode,t.last_episode,jsonb_build_object('edition',w.edition,'plan',p.spec,'source_documents',CASE WHEN $2 THEN snapshot.documents END,'revision_notes',(SELECT jsonb_agg(i->>'gap') FROM scope_heads h JOIN scope_acceptances a ON a.id=h.latest CROSS JOIN jsonb_array_elements(a.checks) c CROSS JOIN jsonb_array_elements(c->'items') i WHERE h.topic=t.id AND a.research_run IS NOT NULL AND c->>'name'<>'图片' AND i?'gap'),'first_episode',t.first_episode,'last_episode',t.last_episode,'new_cast',(SELECT people FROM episode_extensions WHERE topic=t.id),
 'characters',(SELECT jsonb_agg(jsonb_build_array(name,original_name,role) ORDER BY position) FROM work_characters WHERE v=w.version_id),
 'scope',(SELECT jsonb_agg(jsonb_build_array(episode,name,intro) ORDER BY episode) FROM work_chapters WHERE v=w.version_id AND episode BETWEEN t.first_episode AND t.last_episode))
 FROM research_topics t JOIN published_worlds w ON w.slug=t.world_slug
 LEFT JOIN episode_plans p ON p.topic=t.id
 LEFT JOIN episode_source_captures snapshot ON snapshot.topic=p.topic AND snapshot.spec=p.spec
 LEFT JOIN LATERAL (SELECT * FROM research_runs WHERE topic=t.id AND policy=t.policy AND v=w.version_id ORDER BY attempt DESC LIMIT 1) last ON true
 WHERE t.enabled AND (NOT $2 OR snapshot.captured_at>now()-interval '24 hours' OR last.stage='write') AND (p.id IS NOT NULL OR ((t.id,t.first_episode,t.last_episode) IN (('bebop-tv-1998-01-02',1,2),('bebop-tv-1998-03',3,3)))) AND (last.id IS NULL OR last.stage IN ('collect','write') OR
 (last.stage='failed' AND last.attempt<3 AND ($1 OR last.finished_at < now()-interval '5 minutes')))
 ORDER BY (SELECT priority FROM episode_policies WHERE world_slug=t.world_slug) DESC NULLS LAST,t.first_episode,t.id LIMIT 1
