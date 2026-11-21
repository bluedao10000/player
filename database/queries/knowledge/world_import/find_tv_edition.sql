SELECT DISTINCT w.id::text FROM works w JOIN work_sources s ON w.id=s.work_id WHERE w.active AND w.type='TV' AND w.season_year=1998
   AND s.provider='bangumi' AND s.external_id='253' GROUP BY w.id HAVING (SELECT count(DISTINCT x.work_id) FROM work_sources x WHERE x.provider='bangumi' AND x.external_id='253')=1
