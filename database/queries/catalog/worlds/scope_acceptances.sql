SELECT coalesce(jsonb_agg(jsonb_build_object('id',a.id,'world',a.edition_slug,'edition_title',w.name||' · '||w.edition,'first',a.scope_first,'last',a.scope_last,
			'version',a.v,'rule',a.rule,'checked_at',a.checked_at,'published',p.id IS NOT NULL,'passed',a.passed,'checks',a.checks,'picks',a.picks,
            'previous_gaps',(SELECT jsonb_build_object('id',b.id,'items',(SELECT jsonb_agg((c->>'name')||' · '||(i->>'label')||'：'||(i->>'gap') ORDER BY dim,seq)
              FROM jsonb_array_elements(b.checks) WITH ORDINALITY dims(c,dim) CROSS JOIN jsonb_array_elements(c->'items') WITH ORDINALITY items(i,seq) WHERE i ? 'gap'))
              FROM scope_acceptances b WHERE b.topic=a.topic AND b.v=a.v AND b.rule=a.rule AND b.id<a.id AND NOT b.passed ORDER BY b.id DESC LIMIT 1)) ORDER BY h.topic),'[]')
			FROM scope_heads h JOIN scope_acceptances a ON h.latest=a.id JOIN research_topics t ON t.id=h.topic JOIN work_worlds w ON w.slug=t.world_slug LEFT JOIN visible_reader_scopes p ON p.id=a.id
