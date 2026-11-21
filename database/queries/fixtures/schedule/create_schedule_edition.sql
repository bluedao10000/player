WITH k AS (INSERT INTO works(canonical_title,type,season_year) VALUES('fixture','TV',1998) RETURNING id) INSERT INTO work_sources SELECT id,'fixture','1','https://example.com' FROM k
