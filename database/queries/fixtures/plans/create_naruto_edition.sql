WITH k AS (INSERT INTO works(canonical_title,type,season_year) VALUES('fixture','TV',2002) RETURNING id) INSERT INTO work_sources SELECT id,'bangumi','3425','https://bgm.tv/subject/3425' FROM k
