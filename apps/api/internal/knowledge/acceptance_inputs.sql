SELECT jsonb_build_object(
 'world',w.slug,'version',w.version_id,'research',r.id,'documents',r.documents,'claims',r.claims,
 'setting',i.setting,
 'art',coalesce((SELECT jsonb_object_agg(a.item_slug,jsonb_build_object('image',a.image,'sha',a.source_sha256,'credit',a.credit,
   'verified',EXISTS(SELECT FROM media_steps g JOIN media_candidates m ON m.slot=g.slot AND m.round=g.round
     JOIN media_steps q ON q.slot=m.slot AND q.round=m.round AND q.step IN ('review','recheck')
     WHERE g.step='generate' AND m.stage='passed' AND (m.batch IN ('bebop','bebop-portrait') OR m.art_job IS NOT NULL) AND g.result->>'sha'=a.source_sha256
     AND g.result->>'reference_sha'=m.reference_sha AND q.result->>'decision'='pass'
     AND q.result->'checks' @> '{"identity":true,"scene":true,"anatomy":true,"clean":true,"crop":true}'
     AND NOT EXISTS(SELECT FROM media_withdrawals x WHERE (x.slot,x.round)=(m.slot,m.round)))))
   FROM edition_reader_art a WHERE a.world_slug=w.slug AND a.kind IN ('work','character') AND a.item_slug=ANY($3::text[])),'{}'),
 'people',coalesce((SELECT jsonb_object_agg(c.slug,jsonb_build_object('name',c.name,'bio',i.people->>c.slug,
   'episodes',ARRAY(SELECT b.episode FROM chapter_appearances x JOIN work_chapters b ON (b.v,b.slug)=(x.v,x.chapter)
     WHERE (x.v,x.person)=(c.v,c.slug) AND b.episode BETWEEN t.first_episode AND t.last_episode ORDER BY b.episode)))
   FROM work_characters c WHERE c.v=w.version_id),'{}'),
 'chapters',coalesce((SELECT jsonb_agg(jsonb_build_object('episode',c.episode,'slug',c.slug,
   'copy',encode(sha256(convert_to(e.passages::text,'UTF8')),'hex'),'passages',e.passages,
   'sourced',r.documents @> e.sources,'linked',EXISTS(SELECT FROM work_storyline_chapters l WHERE (l.v,l.chapter)=(c.v,c.slug)),
   'scene',s.slug,'place',s.place,'block',s.block,'text',s.text) ORDER BY c.episode)
   FROM work_chapters c LEFT JOIN edition_reader_chapters e ON (e.world_slug,e.chapter)=(w.slug,c.slug)
   LEFT JOIN work_scenes s ON (s.v,s.chapter)=(c.v,c.slug) AND s.slug=CASE c.episode WHEN 1 THEN 'bar-visit' ELSE 'chinatown-chase' END
   WHERE c.v=w.version_id AND c.episode BETWEEN t.first_episode AND t.last_episode),'[]'),
 'images',coalesce((SELECT jsonb_agg(jsonb_build_object('slot',c.slot,'position',c.position,'variant',c.variant,'round',c.round,'name',c.scene,
   'ref',c.reference_sha,'stage',c.stage,'withdrawn',x.reason,'generated',g.result,'review',q.result,'crop',cr.result->>'phone')
   ORDER BY c.position)
   FROM media_candidates c LEFT JOIN media_withdrawals x ON (x.slot,x.round)=(c.slot,c.round)
   LEFT JOIN LATERAL(SELECT result FROM media_steps WHERE slot=c.slot AND round=c.round AND step='generate' AND finished_at IS NOT NULL AND error IS NULL ORDER BY attempt DESC LIMIT 1) g ON true
   LEFT JOIN LATERAL(SELECT result FROM media_steps WHERE slot=c.slot AND round=c.round AND step IN ('review','recheck') AND finished_at IS NOT NULL AND error IS NULL ORDER BY (step='recheck') DESC,attempt DESC LIMIT 1) q ON true
   LEFT JOIN LATERAL(SELECT result FROM media_steps WHERE slot=c.slot AND round=c.round AND step='recrop' AND finished_at IS NOT NULL AND error IS NULL ORDER BY attempt DESC LIMIT 1) cr ON true
   WHERE c.batch=$2),'[]'))
FROM research_topics t JOIN published_worlds w ON w.slug=t.world_slug
LEFT JOIN edition_reader_intros i ON i.world_slug=w.slug
LEFT JOIN research_runs r ON (r.topic,r.v,r.policy)=(t.id,w.version_id,t.policy) AND r.stage='ready'
WHERE t.id=$1 AND t.enabled AND (t.first_episode,t.last_episode)=($4,$5)
