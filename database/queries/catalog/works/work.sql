SELECT to_jsonb(w)||jsonb_build_object(
   'world_href',ww.card->>'href','cover_image',ww.card->>'hero_asset_key',
   'titles',coalesce((SELECT jsonb_agg(to_jsonb(t)-'work_id' ORDER BY lang,title) FROM work_titles t WHERE work_id=w.id),'[]'),
   'sources',coalesce((SELECT jsonb_agg(to_jsonb(s)-'work_id' ORDER BY provider,external_id,url) FROM work_sources s WHERE work_id=w.id),'[]'),
   'tags',coalesce((SELECT jsonb_agg(name ORDER BY name) FROM work_tags WHERE work_id=w.id),'[]'),
   'studios',coalesce((SELECT jsonb_agg(name ORDER BY name) FROM work_studios WHERE work_id=w.id),'[]'),
   'producers',coalesce((SELECT jsonb_agg(name ORDER BY name) FROM work_producers WHERE work_id=w.id),'[]'),
   'broadcasts',coalesce((SELECT jsonb_agg(detail ORDER BY detail) FROM work_broadcasts WHERE work_id=w.id),'[]'),
   'relations',coalesce((SELECT jsonb_agg(to_jsonb(x)-'work_id' ORDER BY url) FROM work_relations x WHERE work_id=w.id),'[]'),
   'provenance',coalesce((SELECT jsonb_agg(jsonb_build_object('source',s.name,'repository',s.repository_url,'version',v.version,'license',v.metadata->'license') ORDER BY s.name,r.record_key)
    FROM knowledge_source_records r JOIN knowledge_sources s ON s.id=r.source_id JOIN knowledge_source_versions v ON v.id=r.version_id WHERE r.active AND r.work_id=w.id),'[]')
   ) FROM works w LEFT JOIN published_worlds ww ON ww.work_id=w.id WHERE w.id=$1 AND w.active
