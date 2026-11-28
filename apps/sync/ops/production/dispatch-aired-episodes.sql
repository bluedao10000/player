\set ON_ERROR_STOP on
BEGIN;
SELECT pg_advisory_xact_lock(82045137);

-- Record the local five-dimensional result independently of the public continuity gate.
UPDATE episode_production_requests r SET stage='local_ready',updated_at=now()
FROM episode_plans p JOIN scope_heads h ON h.topic=p.topic JOIN scope_acceptances a ON a.id=h.latest
WHERE r.production_policy->>'queue'='gz-20-aired-20261125' AND r.stage='executing'
  AND r.reserved=0 AND NOT r.operator_paused AND r.paused_reason IS NULL AND a.passed
  AND p.world_slug=r.world AND p.number=r.first_episode AND p.spec @> '{"publishToReader":false}';

WITH slot AS (
  SELECT extract(epoch FROM date_bin('30 minutes',now(),'2000-01-01 UTC'::timestamptz))::bigint AS id
), candidates AS (
  SELECT r.id,r.xmin AS observed,r.world,r.first_episode,s.id AS slot,
    r.production_policy AS policy, cfg.error,
    CASE
      WHEN p.topic IS NULL AND r.operator_paused AND r.stage IN ('inventory','blocked')
        AND r.paused_reason IN ('等待广州批次','待补可复用的同版本来源模板')
        THEN CASE WHEN r.production_policy @> '{"repair_source":true}' THEN 'source'
          WHEN r.production_policy ? 'source_bundle' OR EXISTS (
          SELECT FROM episode_plans t WHERE t.world_slug=r.world AND t.number<r.first_episode
            AND t.spec->'sourceRules'->>'guide'='wiki-episode' AND t.spec ? 'art'
            AND t.spec->>'guide' LIKE '%#ep'||t.number) THEN 'admit' ELSE 'source' END
      WHEN p.topic IS NULL AND r.stage='blocked' AND NOT r.operator_paused AND r.paused_reason IS NULL
        AND cfg.error IN ('来源标题未唯一命中目标分集','指南正文不足以定位四个独立剧情节点，需要完整分集文章','来源分集标题与目录不一致') THEN 'source'
      WHEN p.topic IS NULL AND r.stage='blocked' AND NOT r.operator_paused AND r.paused_reason IS NULL
        AND cfg.error IS NOT NULL AND r.consumed+16000<=r.token_limit THEN 'configuration'
      WHEN r.stage='executing' AND NOT r.operator_paused AND (r.paused_reason IS NULL OR r.paused_reason='missing_usage_receipt')
        AND a.passed=false AND EXISTS (SELECT FROM jsonb_array_elements(a.checks) c,
          jsonb_array_elements(c->'items') i WHERE c->>'name'='图片' AND i ? 'gap') THEN 'image'
    END AS action
  FROM episode_production_requests r CROSS JOIN slot s
  LEFT JOIN episode_plans p ON p.world_slug=r.world AND p.number=r.first_episode
  LEFT JOIN scope_heads h ON h.topic=p.topic
  LEFT JOIN scope_acceptances a ON a.id=h.latest
  LEFT JOIN LATERAL (SELECT error FROM episode_configuration_attempts WHERE request_id=r.id
    ORDER BY started_at DESC LIMIT 1) cfg ON true
  WHERE r.production_policy->>'queue'='gz-20-aired-20261125' AND (r.reserved=0 OR (r.stage='executing' AND r.paused_reason='missing_usage_receipt'))
    AND NOT EXISTS (SELECT FROM episode_production_requests other
      WHERE other.production_policy->>'queue'='gz-20-aired-20261125'
      AND other.production_policy->>'dispatch_slot'=s.id::text)
), attempts AS (
  SELECT c.*,coalesce((policy->'recovery_counts'->>action)::int,
    CASE WHEN policy->>'recovery_action'=action THEN (policy->>'recovery_count')::int END,0) AS tries
  FROM candidates c WHERE action IS NOT NULL
), ranked AS (
  SELECT c.*,row_number() OVER (PARTITION BY action ORDER BY tries,first_episode,world) AS lane_rank
  FROM attempts c WHERE tries<3 OR action='admit'
), batch AS (
  SELECT * FROM ranked ORDER BY lane_rank,tries,action,first_episode,world LIMIT 5
)
UPDATE episode_production_requests r
SET stage=CASE b.action WHEN 'admit' THEN 'inventory' WHEN 'configuration' THEN 'plan_validated' ELSE r.stage END,
    operator_paused=CASE b.action WHEN 'admit' THEN false WHEN 'source' THEN true ELSE r.operator_paused END,
    paused_reason=CASE b.action WHEN 'admit' THEN NULL WHEN 'source' THEN '待补可复用的同版本来源模板' ELSE r.paused_reason END,
    plan_brief=CASE WHEN b.action='configuration' THEN split_part(coalesce(r.plan_brief,''),E'\n上次配置校验：',1)||E'\n上次配置校验：'||left(b.error,1200) ELSE r.plan_brief END,
    production_policy=r.production_policy||jsonb_build_object('dispatch_slot',b.slot,'recovery_action',b.action,
      'repair_source',b.action='source', 'recovery_count',b.tries+1,'recovery_counts',coalesce(r.production_policy->'recovery_counts','{}')||jsonb_build_object(b.action,b.tries+1),
      'dispatch_history',coalesce(r.production_policy->'dispatch_history','[]')||jsonb_build_array(jsonb_build_object('slot',b.slot,'action',b.action))),
    updated_at=now()
FROM batch b WHERE r.id=b.id AND r.xmin=b.observed
RETURNING r.world,r.first_episode,b.action;
COMMIT;
