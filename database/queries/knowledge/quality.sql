CREATE TEMP TABLE evidence ON COMMIT DROP AS
WITH records AS (
 SELECT r.work_id,string_agg(DISTINCT s.name,', ' ORDER BY s.name) sources,bool_or(length(trim(r.raw->>'picture'))>0) cover,bool_or(s.priority>=50) trusted
 FROM knowledge_source_records r JOIN scope USING(work_id) JOIN knowledge_sources s ON s.id=r.source_id WHERE r.active GROUP BY 1
), aliases AS (
 SELECT r.work_id,t.lang,lower(trim(t.title)) alias,count(*) repetitions
 FROM knowledge_source_records r JOIN scope USING(work_id)
 CROSS JOIN LATERAL jsonb_to_recordset(r.norm->'titles') t(lang text,title text) WHERE r.active
 GROUP BY r.work_id,r.id,t.lang,lower(trim(t.title))
), titles AS (
 SELECT work_id,bool_or(lang~*'^zh($|[-_])') chinese,min(lang) FILTER(WHERE lang !~ '^[a-z]{2,3}(-([A-Z][a-z]{3}|[A-Z]{2}|[0-9]{3}))*$') bad_lang,
 min(alias) FILTER(WHERE repetitions>1) duplicate FROM aliases GROUP BY 1
), provider AS (
 SELECT s.*, count(*) OVER(PARTITION BY provider,external_id) shared FROM (SELECT DISTINCT work_id,provider,external_id FROM work_sources) s
), refs AS (
 SELECT work_id,count(*) ids,min(provider||':'||external_id) FILTER(WHERE shared>1) shared_id FROM provider JOIN scope USING(work_id) GROUP BY 1
), conflicts AS (
 SELECT work_id,min(provider) conflict FROM (SELECT work_id,provider FROM provider JOIN scope USING(work_id) GROUP BY 1,2 HAVING count(*)>1) s GROUP BY 1
), urls AS (
 SELECT work_id,min(url) bad_url FROM work_sources JOIN scope USING(work_id) WHERE url !~* '^https?://[[:alnum:]][^/[:space:]]*([/?#][^[:space:]]*)?$' GROUP BY 1
), broken AS (
 SELECT r.work_id,min(url) broken_url FROM work_relations r JOIN scope USING(work_id) WHERE related_work_id IS NULL GROUP BY 1
)
SELECT w.*,sources,cover,trusted,chinese,bad_lang,duplicate,ids,shared_id,conflict,bad_url,broken_url FROM works w JOIN scope ON w.id=scope.work_id
 LEFT JOIN records USING(work_id) LEFT JOIN titles USING(work_id) LEFT JOIN refs USING(work_id)
 LEFT JOIN conflicts USING(work_id) LEFT JOIN urls USING(work_id) LEFT JOIN broken USING(work_id);
UPDATE knowledge_quality_issues SET status='resolved',checked_at=now() WHERE entity_id IN(SELECT work_id FROM scope) AND status='open';
INSERT INTO knowledge_quality_issues(entity_id,rule_id,field,severity,current_value,suggested_value,source,reason,status)
SELECT e.id,r.rule_id,r.field,r.severity,to_jsonb(v.actual),to_jsonb(v.suggested),coalesce(e.sources,'canonical'),r.reason,'open'
FROM evidence e CROSS JOIN LATERAL (VALUES
 ('required',coalesce(e.type,''),NULL::text,e.type IS NULL OR trim(e.type)=''),
 ('empty_title',e.canonical_title,NULL,trim(e.canonical_title)=''),
 ('url',e.bad_url,NULL,e.bad_url IS NOT NULL),
 ('title_norm',e.canonical_title,regexp_replace(trim(e.canonical_title),'[[:space:]]+',' ','g'),e.canonical_title<>regexp_replace(trim(e.canonical_title),'[[:space:]]+',' ','g')),
 ('language',e.bad_lang,replace(e.bad_lang,'_','-'),e.bad_lang IS NOT NULL),
 ('alias',e.duplicate,NULL,e.duplicate IS NOT NULL),('duplicate_id',e.shared_id,NULL,e.shared_id IS NOT NULL),
 ('conflict',e.conflict,NULL,e.conflict IS NOT NULL),('date',e.end_date::text,NULL,e.end_date<e.start_date),
 ('enum',concat_ws('/',e.type,e.status),NULL,e.type NOT IN ('TV','OVA','ONA','MOVIE','SPECIAL','SHORT_STORY_COLLECTION','UNKNOWN') OR e.status NOT IN ('ONGOING','FINISHED','UPCOMING','UNKNOWN')),
 ('cover',NULL,NULL,NOT coalesce(e.cover,false)),('relation',e.broken_url,NULL,e.broken_url IS NOT NULL),
 ('suspicious',e.episodes::text,NULL,e.episodes<0 OR e.episodes>10000)
) v(rule_id,actual,suggested,fails) JOIN knowledge_quality_rules r USING(rule_id) WHERE v.fails AND r.enabled
ON CONFLICT(entity_id,rule_id) DO UPDATE SET status='open',current_value=excluded.current_value,suggested_value=excluded.suggested_value,
 source=excluded.source,field=excluded.field,severity=excluded.severity,reason=excluded.reason,checked_at=now();
INSERT INTO knowledge_quality_scores
SELECT id,(SELECT sum(value::int) FROM jsonb_each_text(parts)),parts,now() FROM (
 SELECT id,jsonb_build_object('title',CASE WHEN trim(canonical_title)<>'' THEN 15 ELSE 0 END,
 'chinese_title',CASE WHEN chinese THEN 10 ELSE 0 END,'original_title',CASE WHEN length(trim(original_title))>0 THEN 10 ELSE 0 END,
 'provider',CASE WHEN ids>0 AND shared_id IS NULL AND conflict IS NULL THEN 15 ELSE 0 END,
 'date',CASE WHEN start_date IS NOT NULL AND (end_date IS NULL OR end_date>=start_date) THEN 10 ELSE 0 END,
 'type',CASE WHEN type IN ('TV','OVA','ONA','MOVIE','SPECIAL','SHORT_STORY_COLLECTION') THEN 10 ELSE 0 END,'approved_image',0,
 'source',CASE WHEN trusted THEN 10 ELSE 0 END,'relations',CASE WHEN broken_url IS NULL THEN 10 ELSE 0 END) parts FROM evidence
) scored ON CONFLICT(work_id) DO UPDATE SET score=excluded.score,components=excluded.components,checked_at=now();
