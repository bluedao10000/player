\set ON_ERROR_STOP on
BEGIN;
SELECT pg_advisory_xact_lock(82045137);

UPDATE episode_production_requests r
SET stage='local_ready',operator_paused=false,paused_reason=NULL,updated_at=now()
FROM episode_plans p JOIN scope_heads h ON h.topic=p.topic
WHERE r.production_policy->>'queue'='gz-20-aired-20261125'
  AND r.stage IN ('inventory','blocked') AND h.published IS NOT NULL
  AND p.world_slug=r.world AND p.number=r.first_episode;

WITH slot AS (
  SELECT extract(epoch FROM date_bin('30 minutes',now(),'2000-01-01 UTC'::timestamptz))::bigint AS id
), batch AS (
  SELECT r.id,s.id AS slot,r.production_policy ? 'source_bundle' OR t.has_template IS TRUE AS has_template
  FROM episode_production_requests r CROSS JOIN slot s
  LEFT JOIN LATERAL (
    SELECT template.spec->>'guide' LIKE '%#ep'||template.number AS has_template
    FROM episode_plans template
    WHERE template.world_slug=r.world AND template.number<r.first_episode
      AND template.spec->'sourceRules'->>'guide'='wiki-episode' AND template.spec ? 'art'
    ORDER BY template.number DESC LIMIT 1
  ) t ON true
  WHERE r.production_policy->>'queue'='gz-20-aired-20261125'
    AND r.operator_paused AND (
      (r.stage='inventory' AND r.paused_reason='等待广州批次')
      OR (r.stage='blocked' AND r.paused_reason='待补可复用的同版本来源模板'
          AND (r.production_policy ? 'source_bundle' OR t.has_template))
    )
    AND NOT EXISTS (SELECT FROM episode_production_requests prev
      WHERE prev.production_policy->>'queue'='gz-20-aired-20261125'
        AND prev.production_policy->>'dispatch_slot'=s.id::text)
    AND NOT EXISTS (SELECT FROM episode_plans p WHERE p.world_slug=r.world AND p.number=r.first_episode)
  ORDER BY (r.production_policy ? 'source_bundle') DESC,r.first_episode,r.world
  LIMIT 5 FOR UPDATE OF r SKIP LOCKED
)
UPDATE episode_production_requests r
SET stage=CASE WHEN b.has_template THEN 'inventory' ELSE 'blocked' END,
    operator_paused=NOT b.has_template,
    paused_reason=CASE WHEN b.has_template THEN NULL ELSE '待补可复用的同版本来源模板' END,
    production_policy=jsonb_set(r.production_policy,'{dispatch_slot}',to_jsonb(b.slot)),
    updated_at=now()
FROM batch b WHERE r.id=b.id
RETURNING r.world,r.first_episode,r.stage,r.paused_reason;

COMMIT;
