SELECT coalesce(jsonb_agg(s.card ORDER BY s.slug),'[]')
			FROM (SELECT slug,card FROM reader_knowledge_inventory WHERE slug IS NOT NULL ORDER BY slug LIMIT $1) s
