INSERT INTO affected SELECT DISTINCT unnest($1::uuid[])
