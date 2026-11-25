ALTER TABLE workbench_runs
 ADD COLUMN cancel_requested_at timestamptz,
 ADD COLUMN cost_usd numeric CHECK(cost_usd>=0);

CREATE TABLE workbench_artifact_results (
 id uuid NOT NULL DEFAULT gen_random_uuid(),
 kind text NOT NULL CHECK(kind IN ('works-table','coverage-matrix','metric-comparison','cost-breakdown')),
 run_id uuid NOT NULL REFERENCES workbench_runs(id),
 payload jsonb NOT NULL CHECK(jsonb_typeof(payload)='object'),
 PRIMARY KEY(kind,id)
);

-- Only project objects from the currently published version. Base objects are
-- backed by world_evidence foreign keys, and extensions by passed reader scopes.
CREATE VIEW workbench_verified_knowledge AS
WITH objects AS (
 SELECT v,'character' AS kind,slug AS key FROM work_characters
 UNION SELECT v,'character',slug FROM extension_people WHERE length(trim(intro))>0
 UNION ALL SELECT v,'chapter',slug FROM work_chapters
 UNION ALL SELECT v,'chapter',slug FROM published_extensions
 UNION ALL SELECT v,'scene',slug FROM work_scenes
 UNION ALL SELECT v,'scene',scene_slug FROM published_extensions
), entity_counts AS (
 SELECT v,count(DISTINCT (kind,key)) AS verified_entities FROM objects GROUP BY v
), relationships AS (
 SELECT v,count(*) n FROM chapter_appearances GROUP BY v
 UNION ALL SELECT v,count(*) FROM scene_appearances GROUP BY v
), relation_counts AS (
 SELECT v,sum(n) AS n FROM relationships GROUP BY v
)
SELECT p.work_id,p.version_id,coalesce(e.verified_entities,0) AS verified_entities,
 coalesce(e.verified_entities,0)+coalesce(r.n,0) AS verified_items
FROM published_worlds p
LEFT JOIN entity_counts e ON e.v=p.version_id
LEFT JOIN relation_counts r ON r.v=p.version_id;

CREATE VIEW workbench_profiles AS
WITH completeness AS (
 SELECT work_id,
  sum(completed_required_items)::numeric/nullif(sum(applicable_required_items),0) AS completeness
 FROM workbench_current_results WHERE data_state='available' GROUP BY work_id
)
SELECT v.work_id,c.completeness,v.verified_entities AS verified_entity_count
FROM workbench_verified_knowledge v LEFT JOIN completeness c USING(work_id);

-- Price only receipts with unique attribution, using the existing supplier
-- receipt de-duplication, cache breakdown and effective USD cost rates.
CREATE VIEW workbench_usd_receipts AS
SELECT v.subject,v.started_at,v.call_id,v.model,
 CASE WHEN NOT v.multiple_subjects AND v.meter_state NOT IN ('inconsistent','unresolved') THEN v.input_tokens END AS input_tokens,
 CASE WHEN NOT v.multiple_subjects AND v.meter_state NOT IN ('inconsistent','unresolved') THEN v.output_tokens END AS output_tokens,
 CASE WHEN NOT v.multiple_subjects THEN
  billing_amount(rate,v.meter_state,v.total_tokens,v.input_tokens,v.output_tokens,v.images,v.cached)/1000000::numeric
 END AS cost_usd
FROM billing_receipts v
LEFT JOIN LATERAL (
 SELECT r.* FROM billing_rates r WHERE r.kind='cost' AND r.currency='USD'
 AND (r.endpoint,r.provider_account,r.model)=(v.endpoint,v.provider_account,v.model)
 AND r.effective_at<=v.started_at ORDER BY r.effective_at DESC LIMIT 1
) rate ON true;

CREATE VIEW workbench_usd_subjects AS
SELECT subject,CASE WHEN count(*)=count(cost_usd) THEN sum(cost_usd) END AS cost_usd,
 CASE WHEN count(*)=count(input_tokens) THEN sum(input_tokens)::bigint END AS input_tokens,
 CASE WHEN count(*)=count(output_tokens) THEN sum(output_tokens)::bigint END AS output_tokens
FROM workbench_usd_receipts GROUP BY subject;

CREATE FUNCTION workbench_run_json(run jsonb, started timestamptz, finished timestamptz, cost numeric, i bigint, o bigint)
RETURNS jsonb LANGUAGE sql STABLE AS $$
 SELECT (run-'billing_subject')||jsonb_build_object(
  'started_at',workbench_time(started),'finished_at',workbench_time(finished),
  'cost',cost,'currency',CASE WHEN cost IS NOT NULL THEN 'USD' END,'input_tokens',i,'output_tokens',o,
  'usage',jsonb_build_object('input_tokens',i,'output_tokens',o,'cost_usd',cost))
$$;
