-- Inventory for the current reader-visible revision. A scoped acceptance record
-- will be needed before any revision can be reported as formally accepted.
CREATE VIEW work_knowledge_status AS
SELECT k.id AS work_id, coalesce(w.name,k.canonical_title) AS title,
       k.status AS release_status, k.episodes AS total_episodes,
       w.slug, w.edition, w.revision, w.version_id, w.published_at,
       CASE WHEN EXISTS (SELECT FROM scene_appearances a WHERE a.v=w.version_id)
            THEN 'linked' ELSE 'basic' END AS maturity,
       CASE WHEN w.work_id IS NULL THEN 'catalog' ELSE 'sample' END AS visibility,
       (SELECT count(*) FROM work_characters WHERE v=w.version_id) AS characters,
       (SELECT count(DISTINCT person) FROM scene_appearances WHERE v=w.version_id) AS characters_in_scenes,
       (SELECT count(*) FROM work_chapters WHERE v=w.version_id) AS chapters,
       ARRAY(SELECT episode FROM work_chapters WHERE v=w.version_id AND episode IS NOT NULL ORDER BY episode) AS readable_episodes,
       (SELECT count(*) FROM work_storylines WHERE v=w.version_id) AS storylines,
       (SELECT count(*) FROM work_scenes WHERE v=w.version_id) AS scenes,
       (SELECT count(*) FROM work_settings WHERE v=w.version_id) AS settings,
       nullif(w.image,'') IS NOT NULL AS has_hero,
       (SELECT count(*) FROM work_characters WHERE v=w.version_id AND nullif(portrait,'') IS NOT NULL) AS portraits,
       (SELECT count(*) FROM work_chapters WHERE v=w.version_id AND nullif(image,'') IS NOT NULL) AS chapter_images
FROM works k LEFT JOIN work_worlds w ON w.work_id=k.id
WHERE k.active AND (w.version_id IS NOT NULL OR EXISTS (
  SELECT FROM knowledge_source_records r WHERE r.work_id=k.id AND r.active
));
-- scene_appearances has FKs to the evidenced chapter appearance and scene.
-- The scene in turn references an existing block in that same chapter/release.
