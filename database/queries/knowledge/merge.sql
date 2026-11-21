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
CREATE TEMP TABLE incoming_ids ON COMMIT DROP AS SELECT i.record_key,x.provider,array_agg(DISTINCT x.id ORDER BY x.id) ids
 FROM incoming i CROSS JOIN LATERAL jsonb_to_recordset(i.norm->'refs') x(provider text,id text) GROUP BY 1,2;
CREATE TEMP TABLE other_ids ON COMMIT DROP AS SELECT r.work_id,x.provider,array_agg(DISTINCT x.id ORDER BY x.id) ids
 FROM knowledge_source_records r CROSS JOIN merge_context c
 CROSS JOIN LATERAL jsonb_to_recordset(r.norm->'refs') x(provider text,id text)
 WHERE r.active AND r.source_id<>c.source GROUP BY 1,2;
CREATE UNIQUE INDEX ON other_ids(work_id,provider);
ANALYZE incoming_ids;
ANALYZE other_ids;
CREATE TEMP TABLE rejected ON COMMIT DROP AS SELECT DISTINCT m.record_key,m.work_id FROM matches m
 JOIN incoming_ids i ON i.record_key=m.record_key JOIN other_ids o ON o.work_id=m.work_id AND o.provider=i.provider
 WHERE i.ids <> o.ids OR cardinality(i.ids)<>1
 UNION SELECT m.record_key,m.work_id FROM matches m JOIN incoming i USING(record_key)
 JOIN incoming_ids x USING(record_key) JOIN work_sources ws ON ws.work_id=m.work_id AND ws.provider=x.provider
 WHERE m.work_id IS DISTINCT FROM i.old_work AND (cardinality(x.ids)<>1 OR ws.external_id<>ALL(x.ids))
 UNION SELECT m.record_key,m.work_id FROM matches m JOIN incoming i USING(record_key) JOIN knowledge_match_rejections j ON j.record_id=i.record_id AND j.work_id=m.work_id;
CREATE UNIQUE INDEX ON rejected(record_key,work_id);
CREATE TEMP TABLE assignment ON COMMIT DROP AS
 SELECT i.record_key,coalesce(pin.work_id,CASE WHEN rejected.work_id IS NULL OR s.type='github-jsonl' THEN i.old_work END,
   m.only_work,gen_random_uuid()) AS work_id
 FROM incoming i CROSS JOIN merge_context c JOIN knowledge_sources s ON s.id=c.source
 LEFT JOIN knowledge_identity_pins pin ON pin.record_id=i.record_id
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
