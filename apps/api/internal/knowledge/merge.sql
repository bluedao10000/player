-- The caller holds the global merge transaction lock. Merge context has one row.
CREATE TEMP TABLE incoming ON COMMIT DROP AS
 SELECT s.*,r.id AS record_id,r.work_id AS old_work,
 CASE WHEN r.id IS NULL OR NOT r.active THEN 'created'
      WHEN r.raw<>s.raw OR r.norm<>s.norm THEN 'updated' ELSE 'unchanged' END AS action
 FROM staging_source_records s JOIN merge_context c ON s.batch_id=c.batch
 LEFT JOIN knowledge_source_records r ON r.source_id=c.source AND r.record_key=s.record_key;
CREATE UNIQUE INDEX ON incoming(record_key);
ANALYZE incoming;

-- An external identity must identify exactly one canonical work to be auto-linked.
CREATE TEMP TABLE matches ON COMMIT DROP AS
 SELECT DISTINCT i.record_key,w.work_id FROM incoming i
 CROSS JOIN LATERAL jsonb_to_recordset(i.norm->'refs') x(provider text,id text)
 JOIN work_sources w ON (w.provider,w.external_id)=(x.provider,x.id);
CREATE INDEX ON matches(record_key);
-- A common MAL ID does not override a contradictory AniDB season ID.
CREATE TEMP TABLE incoming_ids ON COMMIT DROP AS SELECT i.record_key,x.provider,array_agg(DISTINCT x.id) ids
 FROM incoming i CROSS JOIN LATERAL jsonb_to_recordset(i.norm->'refs') x(provider text,id text) GROUP BY 1,2;
CREATE TEMP TABLE other_ids ON COMMIT DROP AS SELECT r.work_id,x.provider,array_agg(DISTINCT x.id) ids
 FROM knowledge_source_records r CROSS JOIN merge_context c
 CROSS JOIN LATERAL jsonb_to_recordset(r.norm->'refs') x(provider text,id text)
 WHERE r.active AND r.source_id<>c.source GROUP BY 1,2;
CREATE UNIQUE INDEX ON other_ids(work_id,provider);
ANALYZE incoming_ids;
ANALYZE other_ids;
CREATE TEMP TABLE rejected ON COMMIT DROP AS SELECT DISTINCT m.record_key,m.work_id FROM matches m
 JOIN incoming_ids i ON i.record_key=m.record_key JOIN other_ids o ON o.work_id=m.work_id AND o.provider=i.provider
 WHERE NOT (i.ids && o.ids);
CREATE UNIQUE INDEX ON rejected(record_key,work_id);
CREATE TEMP TABLE assignment ON COMMIT DROP AS
 SELECT i.record_key,coalesce(CASE WHEN rejected.work_id IS NULL OR s.type='github-jsonl' THEN i.old_work END,
   m.only_work,gen_random_uuid()) AS work_id
 FROM incoming i CROSS JOIN merge_context c JOIN knowledge_sources s ON s.id=c.source
 LEFT JOIN rejected ON rejected.record_key=i.record_key AND rejected.work_id=i.old_work
 LEFT JOIN (
  SELECT m.record_key,min(m.work_id::text)::uuid AS only_work FROM matches m
  LEFT JOIN rejected r USING(record_key,work_id) GROUP BY m.record_key HAVING count(*)=1 AND count(r.work_id)=0
 ) m ON m.record_key=i.record_key;
CREATE UNIQUE INDEX ON assignment(record_key);
UPDATE incoming i SET action='updated' FROM assignment a WHERE a.record_key=i.record_key AND i.old_work IS NOT NULL AND a.work_id<>i.old_work;
CREATE TEMP TABLE removed ON COMMIT DROP AS SELECT r.id,r.work_id FROM knowledge_source_records r,merge_context c
 WHERE r.source_id=c.source AND r.active AND NOT EXISTS(SELECT FROM incoming i WHERE i.record_key=r.record_key);
CREATE TEMP TABLE affected ON COMMIT DROP AS
 SELECT a.work_id FROM assignment a JOIN incoming i USING(record_key) WHERE i.action<>'unchanged'
 UNION SELECT old_work FROM incoming WHERE action='updated'
 UNION SELECT work_id FROM removed;
CREATE UNIQUE INDEX ON affected(work_id);
ANALYZE affected;
INSERT INTO works(id,canonical_title)
 SELECT DISTINCT ON (a.work_id) a.work_id,i.norm->>'title' FROM assignment a JOIN incoming i USING(record_key)
 ON CONFLICT(id) DO NOTHING;
UPDATE knowledge_source_records SET active=false WHERE id IN (SELECT id FROM removed);
INSERT INTO knowledge_source_records(source_id,record_key,version_id,work_id,raw,norm)
 SELECT c.source,i.record_key,c.version,a.work_id,i.raw,i.norm FROM incoming i JOIN assignment a USING(record_key) CROSS JOIN merge_context c WHERE i.action<>'unchanged'
 ON CONFLICT(source_id,record_key) DO UPDATE SET version_id=excluded.version_id,work_id=excluded.work_id,raw=excluded.raw,norm=excluded.norm,active=true;
UPDATE knowledge_source_records r SET version_id=c.version FROM incoming i,merge_context c
 WHERE r.id=i.record_id AND i.action='unchanged';

CREATE TEMP TABLE contributions ON COMMIT DROP AS
 SELECT r.work_id,r.norm,s.type,r.record_key FROM affected a JOIN knowledge_source_records r USING(work_id)
 JOIN knowledge_sources s ON s.id=r.source_id WHERE r.active;
CREATE INDEX ON contributions(work_id);
ANALYZE contributions;
-- AOD supplies the preferred title, distribution format, episodes and production metadata.
-- Bangumi supplies original/native title and dates. Deterministic tie-break for localized runs.
WITH picked AS (
 SELECT work_id,(array_agg(norm ORDER BY type<>'github-jsonl',record_key))[1] AS main,
 (array_agg(norm ORDER BY record_key) FILTER (WHERE type='npm-dist'))[1] AS bg FROM contributions GROUP BY work_id
)
UPDATE works w SET active=true,canonical_title=main->>'title',original_title=bg->>'original',
 type=main->>'type',episodes=(main->>'episodes')::int,status=main->>'status',
 start_date=(bg->>'start')::date,end_date=(bg->>'end')::date,season=main->>'season',
 season_year=coalesce((main->>'year')::int,(bg->>'year')::int),updated_at=now()
 FROM picked WHERE w.id=picked.work_id;
UPDATE works SET active=false,updated_at=now() WHERE id IN(SELECT work_id FROM affected EXCEPT SELECT work_id FROM contributions);

DELETE FROM work_titles USING affected a WHERE a.work_id=work_titles.work_id;
INSERT INTO work_titles SELECT DISTINCT c.work_id,t.lang,t.title FROM contributions c CROSS JOIN LATERAL jsonb_to_recordset(norm->'titles') t(lang text,title text);
DELETE FROM work_sources USING affected a WHERE a.work_id=work_sources.work_id;
INSERT INTO work_sources SELECT DISTINCT c.work_id,x.provider,x.id,x.url FROM contributions c CROSS JOIN LATERAL jsonb_to_recordset(norm->'refs') x(provider text,id text,url text);
DELETE FROM work_tags USING affected a WHERE a.work_id=work_tags.work_id;
INSERT INTO work_tags SELECT DISTINCT c.work_id,t FROM contributions c CROSS JOIN LATERAL jsonb_array_elements_text(norm->'tags') t;
DELETE FROM work_studios USING affected a WHERE a.work_id=work_studios.work_id;
INSERT INTO work_studios SELECT DISTINCT c.work_id,t FROM contributions c CROSS JOIN LATERAL jsonb_array_elements_text(norm->'studios') t;
DELETE FROM work_producers USING affected a WHERE a.work_id=work_producers.work_id;
INSERT INTO work_producers SELECT DISTINCT c.work_id,t FROM contributions c CROSS JOIN LATERAL jsonb_array_elements_text(norm->'producers') t;
DELETE FROM work_related_urls USING affected a WHERE a.work_id=work_related_urls.work_id;
INSERT INTO work_related_urls SELECT DISTINCT c.work_id,t FROM contributions c CROSS JOIN LATERAL jsonb_array_elements_text(norm->'related') t;
DELETE FROM work_broadcasts USING affected a WHERE a.work_id=work_broadcasts.work_id;
INSERT INTO work_broadcasts SELECT DISTINCT c.work_id,t FROM contributions c CROSS JOIN LATERAL jsonb_array_elements(norm->'broadcasts') t;

-- Rebuild the candidate set across both sources, including evidence introduced on either side.
CREATE TEMP TABLE candidates ON COMMIT DROP AS
 SELECT DISTINCT r.id AS record_id,ws.work_id AS candidate,'provider_conflict' AS reason FROM knowledge_source_records r
 CROSS JOIN LATERAL jsonb_to_recordset(r.norm->'refs') x(provider text,id text)
 JOIN work_sources ws ON ws.provider=x.provider AND ws.external_id=x.id
 WHERE r.active AND ws.work_id<>r.work_id
 UNION ALL
 SELECT DISTINCT r.id,w.id,'title_year' FROM knowledge_source_records r
 JOIN work_titles t ON lower(t.title)=lower(r.norm->>'title') AND t.work_id<>r.work_id
 JOIN works w ON w.id=t.work_id AND w.season_year=(r.norm->>'year')::int
 WHERE r.active AND EXISTS(SELECT 1 FROM knowledge_source_records other WHERE other.active AND other.work_id=w.id AND other.source_id<>r.source_id);
DELETE FROM knowledge_match_candidates k WHERE NOT EXISTS(SELECT FROM candidates c WHERE (c.record_id,c.candidate,c.reason)=(k.source_record_id,k.candidate_work_id,k.reason));
INSERT INTO knowledge_match_candidates(source_record_id,candidate_work_id,reason) SELECT * FROM candidates ON CONFLICT DO NOTHING;
