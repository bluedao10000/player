CREATE TEMP TABLE contributions ON COMMIT DROP AS
 SELECT r.work_id,r.norm,s.type,r.source_id,s.priority,r.record_key FROM affected a JOIN knowledge_source_records r USING(work_id)
 JOIN knowledge_sources s ON s.id=r.source_id WHERE r.active;
CREATE INDEX ON contributions(work_id);
ANALYZE contributions;
-- Choose each non-empty scalar independently, then overlay the human decision.
WITH scalars AS (
 SELECT c.work_id,k.key,k.value,coalesce(p.priority,c.priority) priority,c.record_key,c.source_id
 FROM contributions c CROSS JOIN LATERAL jsonb_each(c.norm) k
 LEFT JOIN knowledge_field_priorities p ON p.source_id=c.source_id AND p.field=k.key
 WHERE k.key IN ('title','original','type','episodes','status','start','end','season','year','synopsis') AND k.value NOT IN ('null','""')
), base AS (
 SELECT work_id,jsonb_object_agg(key,value ORDER BY priority,source_id DESC,record_key DESC) v FROM scalars GROUP BY work_id
), picked AS (
 SELECT work_id,v || coalesce((SELECT jsonb_object_agg(field,value) FROM work_field_overrides o WHERE o.work_id=b.work_id),'{}') n FROM base b
)
UPDATE works w SET active=true,canonical_title=n->>'title',original_title=n->>'original',type=n->>'type',
 episodes=(n->>'episodes')::int,status=n->>'status',start_date=(n->>'start')::date,end_date=(n->>'end')::date,
 season=n->>'season',season_year=coalesce((n->>'year')::int,extract(year from (n->>'start')::date)::int),synopsis=n->>'synopsis',updated_at=now() FROM picked WHERE w.id=picked.work_id;
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
INSERT INTO knowledge_match_candidates(source_record_id,candidate_work_id,reason) SELECT * FROM candidates c WHERE NOT EXISTS(SELECT FROM knowledge_match_rejections j WHERE (j.record_id,j.work_id)=(c.record_id,c.candidate)) ON CONFLICT DO NOTHING;
