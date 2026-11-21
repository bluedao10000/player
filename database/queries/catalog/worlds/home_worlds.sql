SELECT jsonb_build_object('items',coalesce(jsonb_agg(w.card || jsonb_build_object('scene_previews',coalesce((SELECT jsonb_agg(x.card ORDER BY x.slug) FROM (SELECT s.card,s.slug FROM published_scenes s WHERE s.v=w.version_id ORDER BY s.slug LIMIT 4)x),'[]')) ORDER BY h.position),'[]'))
    FROM home_world_slots h JOIN published_worlds w USING(work_id)
