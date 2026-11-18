CREATE VIEW extension_cast AS
 SELECT e.v,e.topic,e.world_slug,p->>'slug' slug,coalesce(b.card,x.card) card,ord
 FROM published_extensions e CROSS JOIN jsonb_array_elements(e.people) WITH ORDINALITY person(p,ord)
 LEFT JOIN published_characters b ON b.v=e.v AND b.slug=p->>'slug'
 LEFT JOIN extension_people x ON x.id=e.id AND x.slug=p->>'slug';
-- Increase only the new topic's writing policy. Existing paired episodes retain their evidence.
UPDATE research_topics SET policy=2 WHERE id='bebop-tv-1998-03' AND policy=1;
