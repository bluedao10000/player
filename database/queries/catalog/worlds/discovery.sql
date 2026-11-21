WITH chapters AS (
			SELECT v,position,card FROM published_chapters
			UNION ALL SELECT v,first_episode,chapter FROM published_extensions
		), scenes AS (
			SELECT s.v,c.position AS chapter,s.position,s.card FROM published_scenes s
			JOIN published_chapters c ON (c.v,c.slug)=(s.v,s.chapter)
			UNION ALL SELECT v,first_episode,0,scene FROM published_extensions
		), people AS (
			SELECT v,0 AS source,position,slug,card FROM published_characters
			UNION ALL SELECT v,1,0,slug,card FROM extension_people
		)
		SELECT jsonb_build_object('items',coalesce(jsonb_agg(w.card || jsonb_build_object(
			'chapters',coalesce(ch.cards,'[]'), 'characters',coalesce(p.cards,'[]'), 'scene',s.card
		) ORDER BY h.position),'[]'))
		FROM home_world_slots h JOIN published_worlds w USING(work_id)
		LEFT JOIN (SELECT DISTINCT ON (v) v,card FROM scenes ORDER BY v,chapter,position,card->>'slug') s ON s.v=w.version_id
		LEFT JOIN (SELECT v,jsonb_agg(card ORDER BY position,card->>'slug') cards FROM chapters GROUP BY v) ch ON ch.v=w.version_id
		LEFT JOIN (SELECT v,jsonb_agg(card ORDER BY source,position,slug) cards FROM people GROUP BY v) p ON p.v=w.version_id
