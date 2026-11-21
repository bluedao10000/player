-- Chapters have their own publication head. Profiles and setting share one head per work.
ALTER TABLE editor_tasks ADD COLUMN kind text NOT NULL DEFAULT 'chapter' CHECK (kind IN ('chapter','profiles'));
ALTER TABLE editor_heads DROP CONSTRAINT editor_heads_pkey, ALTER COLUMN chapter DROP NOT NULL;
ALTER TABLE editor_heads ADD UNIQUE NULLS NOT DISTINCT (world,chapter);

CREATE FUNCTION editor_raw_overview(text) RETURNS jsonb LANGUAGE sql STABLE AS $$
SELECT jsonb_build_object('work',w.card,
    'completed_scope',(SELECT jsonb_build_object('first',min(s.first_episode),'last',max(s.last_episode),
      'picks',jsonb_agg(p-ARRAY['sha','slot','round','variant','phone','phone_sha'] ORDER BY s.first_episode,ord))
      FROM visible_reader_scopes s CROSS JOIN jsonb_array_elements(s.picks) WITH ORDINALITY e(p,ord)
      WHERE s.v=w.version_id AND s.world_slug=w.slug HAVING count(*)>0),
    'reader_intro',coalesce((SELECT jsonb_build_object('premise',i.premise,'setting',i.setting) FROM edition_reader_intros i WHERE i.world_slug=w.slug),
      (SELECT jsonb_build_object('premise',w.intro,'setting',e.reader_copy->e.slug->((p.spec->'setting'->>1)::int-1)->>'text') FROM published_extensions e JOIN episode_plans p USING(topic) WHERE e.v=w.version_id ORDER BY e.first_episode LIMIT 1)),
    'characters',coalesce((SELECT jsonb_agg(card ORDER BY position) FROM published_characters WHERE v=w.version_id),'[]') || coalesce((SELECT jsonb_agg(card ORDER BY slug) FROM extension_people WHERE v=w.version_id),'[]'),
    'storylines',coalesce((SELECT jsonb_agg(jsonb_build_object('slug',l.slug,'title',l.title,'intro',l.intro,'kind','editorial_guide',
      'chapters',coalesce((SELECT jsonb_agg(c.card ORDER BY m.position) FROM work_storyline_chapters m JOIN published_chapters c ON (c.v,c.slug)=(m.v,m.chapter)
          WHERE (m.v,m.line)=(l.v,l.slug)),'[]')) ORDER BY l.position) FROM work_storylines l WHERE l.v=w.version_id),'[]') || coalesce((SELECT jsonb_agg(jsonb_build_object('slug',e.slug,'title',e.scene_name,'intro',e.intro,'kind','editorial_guide','chapters',jsonb_build_array(e.chapter)) ORDER BY e.first_episode) FROM published_extensions e WHERE e.v=w.version_id),'[]'),
    'scenes',coalesce((SELECT jsonb_agg(s.card ORDER BY c.position,s.position) FROM published_scenes s JOIN work_chapters c ON (c.v,c.slug)=(s.v,s.chapter) WHERE s.v=w.version_id),'[]') || coalesce((SELECT jsonb_agg(e.scene ORDER BY e.first_episode) FROM published_extensions e WHERE e.v=w.version_id),'[]'))
    FROM published_worlds w WHERE w.slug=$1
$$;

CREATE FUNCTION editor_raw_person(text,text) RETURNS jsonb LANGUAGE sql STABLE AS $$
SELECT jsonb_build_object('work',w.card,'character',p.card,'intro',coalesce((SELECT i.people->>p.slug FROM edition_reader_intros i WHERE i.world_slug=w.slug), CASE WHEN w.slug='tidepost' AND p.slug='mio' THEN '阿澪熟悉港口的每一盏航灯。她的制服口袋里总装着潮汐表和一枚铜哨，偶尔还有一封舍不得退回的信。' END),'scene_previews',coalesce((SELECT jsonb_agg(s.card ORDER BY s.slug) FROM scene_appearances a JOIN published_scenes s ON (a.v,a.scene)=(s.v,s.slug) WHERE (a.v,a.person)=(p.v,p.slug)),'[]') || coalesce((SELECT jsonb_agg(e.scene) FROM published_extensions e WHERE e.v=p.v AND p.slug='spike'),'[]'),'appearances',coalesce((
    SELECT jsonb_agg(c.card ORDER BY c.position) FROM chapter_appearances a JOIN published_chapters c ON (c.v,c.slug)=(a.v,a.chapter)
    WHERE (a.v,a.person)=(p.v,p.slug)),'[]') || coalesce((SELECT jsonb_agg(e.chapter ORDER BY e.first_episode) FROM published_extensions e WHERE e.v=p.v AND e.people @> jsonb_build_array(jsonb_build_object('slug',p.slug))),'[]')) FROM published_worlds w JOIN published_characters p ON p.v=w.version_id WHERE w.slug=$1 AND p.slug=$2
    UNION ALL SELECT jsonb_build_object('work',w.card,'character',p.card,'intro',p.intro,
      'scene_previews',coalesce((SELECT jsonb_agg(x.scene ORDER BY x.first_episode) FROM published_extensions x JOIN episode_plans plan USING(topic) WHERE x.v=p.v AND x.people @> jsonb_build_array(jsonb_build_object('slug',p.slug)) AND strpos(plan.spec->'art'->>'brief',p.card->>'name')>0),'[]'),
      'appearances',(SELECT jsonb_agg(x.chapter ORDER BY x.first_episode) FROM published_extensions x WHERE x.v=p.v AND x.people @> jsonb_build_array(jsonb_build_object('slug',p.slug))))
    FROM extension_people p JOIN published_extensions e ON e.id=p.id JOIN published_worlds w ON w.version_id=p.v WHERE w.slug=$1 AND p.slug=$2
$$;


CREATE FUNCTION editor_profile_text(p jsonb,k text) RETURNS text LANGUAGE sql IMMUTABLE AS $$
 SELECT coalesce(CASE WHEN k='world/setting' THEN coalesce(p#>>'{reader_intro,setting}',p->>'world_setting') ELSE p->>'intro' END,'')
$$;

CREATE FUNCTION editor_render_profile(p jsonb,k text,patch jsonb) RETURNS jsonb LANGUAGE sql IMMUTABLE AS $$
 SELECT CASE WHEN patch->k->>'original'=editor_profile_text(p,k) THEN
   jsonb_set(p,CASE WHEN k<>'world/setting' THEN ARRAY['intro']
      WHEN jsonb_typeof(p->'reader_intro')='object' THEN ARRAY['reader_intro','setting']
      ELSE ARRAY['world_setting'] END,patch->k->'text') ELSE p END
$$;

CREATE FUNCTION editor_public_profile(p jsonb,w text,k text) RETURNS jsonb LANGUAGE sql STABLE AS $$
 SELECT editor_render_profile(p,k,(SELECT patch FROM editor_heads WHERE world=w AND chapter IS NULL))
$$;

-- Determine editable people from the published appearance relation of the selected chapter.
CREATE FUNCTION editor_task_snapshot(w text,c text,k text) RETURNS jsonb LANGUAGE plpgsql STABLE AS $$
BEGIN
 IF k='chapter' THEN RETURN editor_snapshot(w,c); END IF;
 RETURN (
   WITH chapter AS (SELECT editor_raw_chapter(w,c) page),
   overview AS (SELECT editor_raw_overview(w) page),
   pages(field,page) AS (
     SELECT 'world/setting',page FROM overview WHERE page IS NOT NULL
     UNION ALL
     SELECT 'person/'||(person->>'slug'),p FROM overview o
       CROSS JOIN jsonb_array_elements(o.page->'characters') person
       CROSS JOIN LATERAL editor_raw_person(w,person->>'slug') p
       WHERE p->'appearances' @> jsonb_build_array(jsonb_build_object('slug',c))
   ), visible AS (SELECT field,editor_public_profile(page,w,field) page FROM pages)
   SELECT jsonb_build_object('profile_pages',jsonb_object_agg(field,page),'fields',jsonb_object_agg(field,editor_profile_text(page,field)),
     'research_run',(SELECT page#>>'{research,run_id}' FROM chapter))
   FROM visible HAVING count(*)>0 AND (SELECT page IS NOT NULL FROM chapter)
 );
END $$;

CREATE FUNCTION editor_profile_preview(base jsonb,draft jsonb) RETURNS jsonb LANGUAGE sql IMMUTABLE AS $$
 WITH patch AS (SELECT jsonb_object_agg(d->>'key',jsonb_build_object('text',d->'text','original',base->'fields'->(d->>'key'))) edits
   FROM jsonb_array_elements(draft) d)
 SELECT jsonb_build_object('profile_pages',jsonb_object_agg(key,editor_render_profile(value,key,edits))) FROM jsonb_each(base->'profile_pages'),patch
$$;
