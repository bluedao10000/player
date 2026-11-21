INSERT INTO works(canonical_title,type,synopsis) VALUES($1,'SHORT_STORY_COLLECTION',$2) RETURNING id::text
