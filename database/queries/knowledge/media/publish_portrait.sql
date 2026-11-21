INSERT INTO edition_reader_art(world_slug,kind,item_slug,image,caption,credit,source_sha256)
				SELECT coalesce(a.world_slug,'cowboy-bebop-tv'),'character',coalesce(a.slug,c.character_slug),
				       '/art/portraits/'||(g.result->>'public_sha')||'.webp',c.scene,'AI 人物肖像',g.result->>'sha'
				FROM media_candidates c LEFT JOIN episode_art_jobs a ON a.id=c.art_job JOIN media_steps g ON g.slot=c.slot AND g.round=c.round AND g.step='generate'
				JOIN media_steps r ON r.slot=c.slot AND r.round=c.round AND r.step=$2 AND r.attempt=$3
				WHERE c.slot=$1 AND (a.kind='portrait' OR c.batch='bebop-portrait') AND g.error IS NULL AND g.finished_at IS NOT NULL
				  AND g.result->>'public_sha' ~ '^[0-9a-f]{64}$' AND g.result->>'reference_sha'=c.reference_sha
				  AND r.result->>'decision'='pass' AND r.result->>'phone'=g.result->>'phone'
				  AND r.result->'checks' @> '{"identity":true,"scene":true,"anatomy":true,"clean":true,"crop":true}'
				ON CONFLICT (world_slug,kind,item_slug) DO UPDATE SET image=excluded.image,caption=excluded.caption,
				    credit=excluded.credit,source_sha256=excluded.source_sha256
