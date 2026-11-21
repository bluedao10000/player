INSERT INTO world_fetch_cache VALUES($1,$2,$3,$4,$5) ON CONFLICT(source_id,url) DO UPDATE SET etag=excluded.etag,modified=excluded.modified,body=excluded.body
