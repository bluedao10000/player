WITH page AS MATERIALIZED (
   SELECT w.id,w.canonical_title,w.original_title,w.type,w.episodes,w.status,w.season_year,
     ww.card->>'href' world_href,ww.card->>'hero_asset_key' cover_image
   FROM works w LEFT JOIN published_worlds ww ON ww.work_id=w.id WHERE w.active AND ($1='' OR strpos(lower(w.canonical_title),lower($1))>0 OR strpos(lower(w.original_title),lower($1))>0 OR EXISTS(SELECT FROM work_titles t WHERE t.work_id=w.id AND strpos(lower(t.title),lower($1))>0))
   AND (nullif($2,'') IS NULL OR w.id>nullif($2,'')::uuid) ORDER BY w.id LIMIT $3+1
  ), visible AS(SELECT * FROM page ORDER BY id LIMIT $3)
  SELECT jsonb_build_object('items',coalesce(jsonb_agg(to_jsonb(visible) ORDER BY id),'[]'),
   'next_cursor',CASE WHEN (SELECT count(*) FROM page)>$3 THEN max(id::text) END) FROM visible
