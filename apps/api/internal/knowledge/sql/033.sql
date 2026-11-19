CREATE TABLE editor_tasks (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), request_id uuid NOT NULL UNIQUE, world text NOT NULL, chapter text NOT NULL,
 instruction text NOT NULL, author text NOT NULL, created_at timestamptz NOT NULL DEFAULT now(),
 status text NOT NULL DEFAULT 'evidence' CHECK(status IN ('evidence','drafting','checking','review','published','failed','conflict')),
 round integer NOT NULL DEFAULT 1, attempt integer NOT NULL DEFAULT 1, retries integer NOT NULL DEFAULT 0,
 lease uuid, due timestamptz NOT NULL DEFAULT now(), base_head uuid, base jsonb NOT NULL, sources jsonb NOT NULL DEFAULT '[]',
 draft jsonb NOT NULL DEFAULT '[]', feedback text NOT NULL DEFAULT '', issue text NOT NULL DEFAULT '',
 reviewer text, published_at timestamptz
);
CREATE INDEX editor_queue ON editor_tasks(due) WHERE status IN ('evidence','drafting','checking');
CREATE TABLE editor_events(id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,task uuid NOT NULL REFERENCES editor_tasks, at timestamptz NOT NULL DEFAULT now(), round integer NOT NULL,attempt integer NOT NULL,stage text NOT NULL,note text NOT NULL,detail jsonb);
CREATE INDEX editor_task_history ON editor_events(task,id);
CREATE TABLE editor_heads(world text NOT NULL,chapter text NOT NULL, task uuid NOT NULL REFERENCES editor_tasks, patch jsonb NOT NULL, PRIMARY KEY(world,chapter));

CREATE FUNCTION editor_raw_chapter(text,text) RETURNS jsonb LANGUAGE sql STABLE AS $$
SELECT jsonb_build_object('work',w.card,'chapter',c.card,'accepted_art',(SELECT jsonb_agg(p-ARRAY['sha','slot','round','variant','phone','phone_sha'] ORDER BY ord) FROM visible_reader_scopes s CROSS JOIN jsonb_array_elements(s.picks) WITH ORDINALITY e(p,ord) WHERE s.world_slug=w.slug AND s.v=c.v AND c.episode BETWEEN s.first_episode AND s.last_episode AND p->>'chapter'=c.slug),
    'content_kind',CASE w.origin WHEN 'original' THEN 'original' ELSE 'editorial_guide' END,
    'research',(SELECT jsonb_build_object('run_id',r.run_id,'paragraphs',r.paragraphs)
      FROM chapter_research r WHERE (r.v,r.slug)=(c.v,c.slug) ORDER BY r.run_id LIMIT 1),
    'reader_passages',coalesce((SELECT s.reader_copy->c.slug FROM visible_reader_scopes s WHERE s.v=c.v AND s.world_slug=w.slug AND c.episode BETWEEN s.first_episode AND s.last_episode), (SELECT jsonb_agg(jsonb_build_object('key',coalesce(p->>'block','episode-'||ord::text),'text',p->>'text',
      'anchors',coalesce((SELECT jsonb_agg('passage-'||s.slug ORDER BY s.position) FROM work_scenes s
        WHERE (s.v,s.chapter,s.block)=(c.v,c.slug,p->>'block')),'[]')) ORDER BY ord)
      FROM edition_reader_chapters i JOIN chapter_research cr ON (cr.v,cr.slug)=(c.v,c.slug)
      JOIN research_runs rr ON rr.id=cr.run_id AND rr.documents @> i.sources
      CROSS JOIN jsonb_array_elements(i.passages) WITH ORDINALITY a(p,ord)
      WHERE (i.world_slug,i.chapter)=(w.slug,c.slug))),
    'next_chapter',coalesce((SELECT n.card FROM published_chapters n WHERE n.v=c.v AND n.position>c.position ORDER BY n.position LIMIT 1),(SELECT e.chapter FROM published_extensions e WHERE e.v=c.v AND e.first_episode=c.episode+1)),
    'blocks',(SELECT jsonb_agg(jsonb_build_object('key',b.key,'text',b.text,'anchors',coalesce((SELECT jsonb_agg('passage-'||s.slug ORDER BY s.position) FROM work_scenes s
      WHERE (s.v,s.chapter,s.block)=(b.v,b.chapter,b.key)),'[]')) ORDER BY b.position) FROM chapter_blocks b WHERE (b.v,b.chapter)=(c.v,c.slug)),
    'scenes',coalesce((SELECT jsonb_agg(s.card ORDER BY s.position) FROM published_scenes s WHERE (s.v,s.chapter)=(c.v,c.slug)),'[]'),
    'attribution', (SELECT jsonb_agg(jsonb_build_object('uri',e.uri,'locator',e.locator) ORDER BY e.key) FROM world_evidence e WHERE e.v=c.v AND
      (e.key=c.evidence OR EXISTS(SELECT FROM chapter_appearances a WHERE (a.v,a.chapter,a.evidence)=(e.v,c.slug,e.key)))))
    FROM published_worlds w JOIN published_chapters c ON w.version_id=c.v WHERE w.slug=$1 AND c.slug=$2
    UNION ALL SELECT jsonb_build_object('work',w.card,'chapter',e.chapter,'content_kind','editorial_guide',
      'accepted_art',(SELECT jsonb_agg(p-ARRAY['sha','slot','round','variant','phone','phone_sha']) FROM jsonb_array_elements(e.picks) p),
      'research',jsonb_build_object('run_id',r.id,'paragraphs',r.claims),'reader_passages',e.reader_copy->e.slug,
      'blocks',e.reader_copy->e.slug,'scenes',jsonb_build_array(e.scene),'next_chapter',(SELECT x.chapter FROM published_extensions x WHERE x.v=e.v AND x.first_episode=e.last_episode+1),'attribution','[]'::jsonb)
    FROM published_extensions e JOIN published_worlds w ON w.version_id=e.v JOIN research_runs r ON r.id=e.research_run WHERE w.slug=$1 AND e.slug=$2
$$;
CREATE FUNCTION editor_raw_scene(text,text) RETURNS jsonb LANGUAGE sql STABLE AS $$
SELECT jsonb_build_object('work',w.card,'scene',s.card||jsonb_build_object('text',s.text),'chapter',c.card,'setting',s.place,
    'scene_cast',coalesce((SELECT jsonb_agg(p.card ORDER BY p.position) FROM scene_appearances a JOIN published_characters p ON (p.v,p.slug)=(a.v,a.person)
     WHERE (a.v,a.scene)=(s.v,s.slug)),'[]'),
    'return_to',jsonb_build_object('block_key',s.block,'href',(c.card->>'href')||'#passage-'||s.slug),
    'attribution',(SELECT jsonb_agg(jsonb_build_object('uri',e.uri,'locator',e.locator) ORDER BY e.key) FROM world_evidence e WHERE e.v=s.v AND
      (e.key=s.evidence OR EXISTS(SELECT FROM scene_appearances a WHERE (a.v,a.scene,a.evidence)=(e.v,s.slug,e.key)))))
    FROM published_worlds w JOIN published_scenes s ON s.v=w.version_id JOIN published_chapters c ON (c.v,c.slug)=(s.v,s.chapter) WHERE w.slug=$1 AND s.slug=$2
    UNION ALL SELECT jsonb_build_object('work',w.card,'chapter',e.chapter,'setting',e.place,'scene',e.scene||jsonb_build_object('text',e.reader_copy->e.slug->coalesce((plan.spec->>'sceneBeat')::int-1,1)->>'text'),
      'scene_cast',coalesce((SELECT jsonb_agg(card ORDER BY ord) FROM extension_cast c WHERE c.topic=e.topic AND ((plan.id IS NULL AND c.slug='spike') OR (plan.id IS NOT NULL AND strpos(plan.spec->'art'->>'brief',c.card->>'name')>0))),'[]'::jsonb),
      'return_to',jsonb_build_object('block_key',e.picks->0->>'block','href',(e.chapter->>'href')||'#passage-'||e.scene_slug),'attribution','[]'::jsonb)
    FROM published_extensions e JOIN published_worlds w ON w.version_id=e.v LEFT JOIN episode_plans plan USING(topic) WHERE w.slug=$1 AND e.scene_slug=$2
$$;

-- Only substitute reviewed text whose original still matches. IDs, order, artwork and links always come from the current reader projection.
CREATE FUNCTION editor_render(p jsonb, patch jsonb) RETURNS jsonb LANGUAGE plpgsql IMMUTABLE AS $$
DECLARE field text; entry jsonb; blocks jsonb; key text;
BEGIN
 IF p IS NULL OR patch IS NULL OR patch='{}' THEN RETURN p; END IF;
 IF p ? 'return_to' THEN
   entry:=patch->('scene/'||(p#>>'{scene,slug}'));
   IF entry->>'original'=p#>>'{scene,text}' THEN RETURN jsonb_set(p,'{scene,text}',entry->'text'); END IF;
   RETURN p;
 END IF;
 field:=CASE WHEN jsonb_typeof(p->'reader_passages')='array' THEN 'reader_passages' ELSE 'blocks' END;
 SELECT jsonb_agg(CASE WHEN b->>'text'=patch->('block/'||(b->>'key'))->>'original'
   THEN jsonb_set(b,'{text}',patch->('block/'||(b->>'key'))->'text') ELSE b END ORDER BY n) INTO blocks
 FROM jsonb_array_elements(p->field) WITH ORDINALITY a(b,n);
 IF blocks IS NOT NULL THEN p:=jsonb_set(p,ARRAY[field],blocks); END IF;
 RETURN p;
END $$;
CREATE FUNCTION editor_public_page(p jsonb,w text) RETURNS jsonb LANGUAGE sql STABLE AS $$
 SELECT editor_render(p,(SELECT patch FROM editor_heads WHERE world=w AND chapter=p#>>'{chapter,slug}'))
$$;
CREATE FUNCTION editor_snapshot(w text,c text) RETURNS jsonb LANGUAGE sql STABLE AS $$
 WITH chapter AS (SELECT editor_public_page(editor_raw_chapter(w,c),w) page)
 SELECT jsonb_build_object('chapter',page,'scenes',coalesce((SELECT jsonb_object_agg(s->>'slug',editor_public_page(editor_raw_scene(w,s->>'slug'),w))
 FROM jsonb_array_elements(page->'scenes') s),'{}'::jsonb)) FROM chapter WHERE page IS NOT NULL
$$;
CREATE TABLE editor_sessions(hash bytea PRIMARY KEY,operator text NOT NULL,expires_at timestamptz NOT NULL DEFAULT now()+interval '8 hours');
