-- Keep source-world inventory intact; count only publicly readable extensions.
CREATE VIEW reader_knowledge_inventory AS
 SELECT s.work_id,s.slug,to_jsonb(s)||jsonb_build_object(
   'readable_episodes',s.readable_episodes||x.episodes,'chapters',s.chapters+x.n,
   'characters',s.characters+p.n,'portraits',s.portraits+p.portraits,'chapter_images',s.chapter_images+x.n,
   'storylines',s.storylines+x.n,'scenes',s.scenes+x.n,'settings',s.settings+x.places) AS card
 FROM work_knowledge_status s
 CROSS JOIN LATERAL (SELECT count(*) n,count(DISTINCT place) places,
  coalesce(array_agg(first_episode ORDER BY first_episode),'{}'::int[]) episodes
  FROM published_extensions WHERE v=s.version_id) x
 CROSS JOIN LATERAL (SELECT count(*) n,count(card->>'portrait') portraits FROM extension_people WHERE v=s.version_id) p;
