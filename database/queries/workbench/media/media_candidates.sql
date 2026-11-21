SELECT jsonb_agg(to_jsonb(x) ORDER BY x.slot) FROM (
    SELECT c.*,a.spec AS art,CASE WHEN c.batch='bebop-portrait' AND c.stage='passed' THEN
       (SELECT card->>'href' FROM published_characters WHERE slug=c.character_slug AND v=(SELECT version_id FROM work_worlds WHERE slug='cowboy-bebop-tv')
        UNION ALL SELECT card->>'href' FROM extension_people WHERE slug=c.character_slug AND world_slug='cowboy-bebop-tv' LIMIT 1) END AS reader_href,
       coalesce((SELECT jsonb_agg(to_jsonb(s)-'slot' ORDER BY started_at) FROM media_steps s WHERE s.slot=c.slot),'[]') AS steps
    FROM media_candidates c LEFT JOIN episode_art_jobs a ON a.id=c.art_job)x
