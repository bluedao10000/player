INSERT INTO scope_heads(topic,latest,published) VALUES($1,$2,CASE WHEN $3 THEN $2::bigint END)
   ON CONFLICT(topic) DO UPDATE SET latest=excluded.latest,published=excluded.published WHERE scope_heads.latest IS DISTINCT FROM excluded.latest
