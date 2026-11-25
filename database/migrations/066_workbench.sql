CREATE TABLE workbench_conversations (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 operator text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE workbench_runs (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 conversation_id uuid REFERENCES workbench_conversations(id),
 work_id uuid REFERENCES works(id),
 task_name text NOT NULL,
 task_type text NOT NULL,
 status text NOT NULL CHECK (status IN ('queued','running','completed','failed','cancelled','waiting_approval')),
 started_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 finished_at timestamptz,
 input_tokens bigint CHECK (input_tokens >= 0),
 output_tokens bigint CHECK (output_tokens >= 0),
 tool_count int NOT NULL DEFAULT 0 CHECK (tool_count >= 0),
 result_summary text,
 error_summary text
);
CREATE INDEX workbench_runs_recent ON workbench_runs(started_at DESC,id);
CREATE INDEX workbench_runs_conversation ON workbench_runs(conversation_id,started_at DESC);

CREATE TABLE workbench_run_events (
 run_id uuid NOT NULL REFERENCES workbench_runs(id),
 sequence int NOT NULL CHECK (sequence > 0),
 timestamp timestamptz NOT NULL DEFAULT clock_timestamp(),
 payload jsonb NOT NULL CHECK (jsonb_typeof(payload)='object'),
 PRIMARY KEY (run_id,sequence)
);

CREATE TABLE workbench_knowledge_plans (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 work_id uuid NOT NULL REFERENCES works(id),
 run_id uuid NOT NULL UNIQUE REFERENCES workbench_runs(id),
 title text NOT NULL,
 objective text NOT NULL,
 status text NOT NULL CHECK (status IN ('draft','pending_approval','approved','running','completed','cancelled')),
 revision int NOT NULL DEFAULT 1 CHECK (revision > 0),
 milestones jsonb NOT NULL DEFAULT '[]' CHECK (jsonb_typeof(milestones)='array'),
 tasks jsonb NOT NULL CHECK (jsonb_typeof(tasks)='array'),
 created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE workbench_inbox (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 work_id uuid NOT NULL REFERENCES works(id),
 plan_id uuid UNIQUE REFERENCES workbench_knowledge_plans(id),
 source_review_id bigint UNIQUE REFERENCES knowledge_source_review_marks(id),
 source_run_id uuid REFERENCES knowledge_sync_runs(id),
 run_id uuid REFERENCES workbench_runs(id),
 title text NOT NULL,
 description text NOT NULL,
 assessment text,
 diff jsonb NOT NULL DEFAULT '[]' CHECK (jsonb_typeof(diff)='array'),
 status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','approved','rejected','snoozed','expired')),
 risk_level text NOT NULL CHECK (risk_level IN ('L1','L2','L3')),
 action_type text NOT NULL CHECK (action_type IN ('plan.approve','source.review')),
 revision int NOT NULL DEFAULT 1 CHECK (revision > 0),
 created_at timestamptz NOT NULL DEFAULT now(),
 expires_at timestamptz,
 resolved_by text,
 resolution_note text,
 CHECK (
  (action_type='plan.approve' AND plan_id IS NOT NULL AND source_review_id IS NULL AND source_run_id IS NULL) OR
  (action_type='source.review' AND plan_id IS NULL AND source_review_id IS NOT NULL AND source_run_id IS NOT NULL)
 )
);
CREATE INDEX workbench_inbox_recent ON workbench_inbox(created_at DESC,id);
CREATE UNIQUE INDEX workbench_inbox_receipt ON workbench_inbox(run_id) WHERE run_id IS NOT NULL;

CREATE TABLE workbench_decision_actions (
 decision_id uuid NOT NULL REFERENCES workbench_inbox(id),
 revision int NOT NULL CHECK (revision > 0),
 operator text NOT NULL,
 action text NOT NULL CHECK (action IN ('approve','reject','snooze')),
 note text,
 acted_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY (decision_id,revision)
);

-- A new upstream snapshot revises a pending source review in the same
-- transaction that records the source changes.
CREATE FUNCTION refresh_workbench_source_review() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.resolved_at IS NOT NULL THEN RETURN NEW; END IF;
 INSERT INTO workbench_inbox(work_id,source_review_id,source_run_id,title,description,risk_level,action_type,created_at)
 SELECT w.work_id,NEW.id,NEW.last_run_id,'核对来源变动 · '||w.name,
   '核对上游快照差异并留下处理记录。已发布内容继续沿用当前定稿。',
   'L2','source.review',NEW.updated_at
 FROM work_worlds w WHERE w.slug=NEW.world_slug
 ON CONFLICT(source_review_id) DO UPDATE
 SET source_run_id=excluded.source_run_id,revision=workbench_inbox.revision+1,
     status='pending',resolved_by=NULL,resolution_note=NULL,run_id=NULL
 WHERE workbench_inbox.source_run_id<>excluded.source_run_id;
 RETURN NEW;
END
$$;

INSERT INTO workbench_inbox(work_id,source_review_id,source_run_id,title,description,risk_level,action_type,created_at)
SELECT w.work_id,m.id,m.last_run_id,'核对来源变动 · '||w.name,
 '核对上游快照差异并留下处理记录。已发布内容继续沿用当前定稿。',
 'L2','source.review',m.updated_at
FROM knowledge_source_review_marks m JOIN work_worlds w ON w.slug=m.world_slug
WHERE m.resolved_at IS NULL;

CREATE TRIGGER workbench_source_review_changed
AFTER INSERT OR UPDATE ON knowledge_source_review_marks
FOR EACH ROW EXECUTE FUNCTION refresh_workbench_source_review();

CREATE FUNCTION workbench_time(timestamptz) RETURNS text
LANGUAGE sql STABLE STRICT AS $$
 SELECT to_char($1 AT TIME ZONE 'Asia/Shanghai','YYYY-MM-DD"T"HH24:MI:SS.MS')||'+08:00'
$$;

CREATE VIEW workbench_current_results AS
SELECT r.* FROM knowledge_completeness_results r
JOIN published_worlds w ON (w.work_id,w.version_id)=(r.work_id,r.version_id)
JOIN knowledge_completeness_rules rule USING(rule_code)
WHERE rule.active AND r.rule_version=rule.rule_version;

CREATE VIEW workbench_work_gaps AS
SELECT r.work_id,
  'check:'||r.rule_code||':'||(item->>'item_key') AS id,
  CASE WHEN item->>'status'='pending_verification' THEN 'weak_evidence'
       WHEN r.dimension='images' THEN 'missing_image'
       WHEN r.dimension='timeline' THEN 'timeline_anomaly'
       WHEN r.item_code='relationships' THEN 'incomplete_relation'
       ELSE 'missing_entity' END AS kind,
  rule.label||' · '||(item->>'item_key') AS label,
  coalesce(item->>'reason','知识完整度检查未通过') AS detail,
  CASE WHEN item->>'status'='pending_verification' THEN 'mid' ELSE 'high' END AS severity
FROM workbench_current_results r JOIN knowledge_completeness_rules rule USING(rule_code)
CROSS JOIN LATERAL jsonb_array_elements(r.details) item
WHERE item->>'status' IN ('missing','partial','pending_verification')
UNION ALL
SELECT w.work_id,'source:'||m.id,'source_conflict','核对上游来源变动',
       '来源快照 '||m.source_version_id||' 有待核对的变动。','mid'
FROM knowledge_source_review_marks m JOIN work_worlds w ON w.slug=m.world_slug
WHERE m.resolved_at IS NULL;

CREATE VIEW workbench_work_summary AS
WITH checks AS (
 SELECT work_id,count(*) FILTER(WHERE data_state='available') AS measured,max(calculated_at) AS calculated_at
 FROM workbench_current_results GROUP BY work_id
), gaps AS (
 SELECT work_id,count(*) n FROM workbench_work_gaps GROUP BY work_id
), issues AS (
 SELECT entity_id,count(*) n FROM knowledge_quality_issues WHERE status='open' GROUP BY entity_id
), relations AS (
 SELECT work_id,count(DISTINCT related_work_id) n FROM work_relations
 WHERE related_work_id IS NOT NULL GROUP BY work_id
), works_with_facts AS (
 SELECT w.id AS work_id,w.canonical_title AS title,w.original_title,
   p.card->>'hero_asset_key' AS cover_url,
   CASE w.type WHEN 'TV' THEN 'TV 动画' WHEN 'MOVIE' THEN '剧场版'
       WHEN 'SPECIAL' THEN '特别篇' ELSE w.type END AS work_type,w.season_year AS release_year,
   CASE WHEN NOT w.active THEN 'archived' WHEN p.work_id IS NOT NULL THEN 'published' ELSE 'draft' END AS publication_status,
   NULL::bigint AS verified_entity_count,(inv.card->>'characters')::bigint AS character_count,
   (inv.card->>'chapters')::bigint AS chapter_count,NULL::bigint AS event_count,relations.n AS relation_count,
   CASE WHEN w.type IN ('TV','ONA','OVA','SPECIAL') AND w.episodes>0 AND p.work_id IS NOT NULL
       AND episodes.outside=0 THEN episodes.in_scope::numeric/w.episodes END AS coverage,
   CASE WHEN checks.measured>0 OR gaps.n IS NOT NULL THEN coalesce(gaps.n,0) END AS gap_count,
   CASE WHEN quality.work_id IS NOT NULL OR issues.n IS NOT NULL THEN coalesce(issues.n,0) END AS quality_issue_count,
   (inv.card->>'maturity')='linked' AS linked,
   greatest(w.updated_at,p.published_at,checks.calculated_at,quality.checked_at) AS updated_at
 FROM works w LEFT JOIN published_worlds p ON p.work_id=w.id
 LEFT JOIN reader_knowledge_inventory inv ON inv.work_id=p.work_id
 LEFT JOIN checks ON checks.work_id=w.id LEFT JOIN gaps ON gaps.work_id=w.id
 LEFT JOIN knowledge_quality_scores quality ON quality.work_id=w.id
 LEFT JOIN issues ON issues.entity_id=w.id LEFT JOIN relations ON relations.work_id=w.id
 LEFT JOIN LATERAL (
   SELECT count(DISTINCT n) FILTER(WHERE n BETWEEN 1 AND w.episodes) AS in_scope,
          count(*) FILTER(WHERE n NOT BETWEEN 1 AND w.episodes) AS outside
   FROM jsonb_array_elements_text(inv.card->'readable_episodes') value
   CROSS JOIN LATERAL (SELECT value::int AS n) numbered
 ) episodes ON w.type IN ('TV','ONA','OVA','SPECIAL') AND w.episodes>0 AND p.work_id IS NOT NULL
)
SELECT work_id,title,original_title,cover_url,work_type,release_year,publication_status,
 verified_entity_count,character_count,chapter_count,event_count,relation_count,coverage,
 CASE WHEN coverage IS NULL THEN 'unknown' WHEN coverage=1 THEN 'complete'
      WHEN coverage>0 THEN 'partial' ELSE 'missing' END AS coverage_status,
 gap_count,quality_issue_count,
 CASE WHEN linked AND coverage=1 AND gap_count=0 THEN 'reference'
      WHEN linked THEN 'established' WHEN publication_status='published' THEN 'growing' ELSE 'seed' END AS maturity_level,
 updated_at
FROM works_with_facts;

CREATE VIEW workbench_decision_items AS
SELECT d.id,d.title,d.description,
 CASE WHEN d.status IN ('pending','snoozed') AND d.expires_at<=statement_timestamp() THEN 'expired' ELSE d.status END AS status,
 d.risk_level,d.action_type,
 CASE WHEN d.plan_id IS NOT NULL THEN jsonb_build_object('plan_id',d.plan_id)
      ELSE jsonb_build_object('source_review_id',d.source_review_id,'source_run_id',d.source_run_id) END AS proposal,
 d.revision AS expected_revision,NULL::numeric AS confidence,1 AS affected_works,
 NULL::int AS affected_entities,CASE WHEN d.source_review_id IS NOT NULL THEN 1 END AS affected_records,
 CASE WHEN d.source_review_id IS NOT NULL THEN 1 END AS evidence_count,false AS rollback_available,
 NULL::text AS rollback_summary,d.created_at,d.expires_at,
 coalesce(d.assessment,CASE WHEN d.source_review_id IS NOT NULL THEN
   '此次决定只记录对来源变动的处理结果。公开定稿继续使用已发布版本。' END) AS assessment,
 CASE WHEN d.source_review_id IS NULL THEN d.diff ELSE coalesce((
   SELECT jsonb_agg(jsonb_build_object(
     'field',change.name||'.'||change.key,
     'kind',CASE WHEN change.value->>'before' IS NULL THEN 'add' WHEN change.value->>'after' IS NULL THEN 'del' ELSE 'mod' END,
     'before',change.value->>'before','after',change.value->>'after') ORDER BY change.name,change.key)
   FROM (
    SELECT section.name,change.key,change.value
    FROM (VALUES ('fields'),('catalog')) section(name)
    CROSS JOIN LATERAL jsonb_each(m.changes->section.name) change
    UNION ALL
    SELECT 'digest',key,value FROM jsonb_each(m.changes)
    WHERE key IN ('raw','norm') AND value->'before' IS DISTINCT FROM value->'after'
   ) change
 ),'[]') END AS diff,
 d.work_id,d.run_id,d.resolution_note
FROM workbench_inbox d LEFT JOIN knowledge_source_review_marks m ON m.id=d.source_review_id;
