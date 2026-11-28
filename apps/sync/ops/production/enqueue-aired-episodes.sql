\set ON_ERROR_STOP on
BEGIN;
SELECT pg_advisory_xact_lock(82045137);

WITH targets(slug) AS (VALUES
  ('apothecary-diaries-s3-2026'), ('dark-summoner-on-air-2026'), ('enchanter-second-life-tv-2026'),
  ('fx-fighter-kurumi-tv-2026'), ('horemajo-tv-2026'), ('magical-explorer-tv-2026'),
  ('mushoku-tensei-s3-2026'), ('nia-liston-tv-2026'), ('ramparts-of-ice-s2-2026'),
  ('ranma-half-s3-2026'), ('reborn-as-space-mercenary-tv-2026'), ('reincarnated-as-a-sword-s2-2026'),
  ('romelia-war-chronicle-tv-2026'), ('seitokai-ana-tv-2026'), ('sekamajo-tv-2026'),
  ('smoking-behind-the-supermarket-tv-2026'), ('snow-widow-cursed-ring-tv-2026'), ('toaru-item-tv-2026'),
  ('wild-last-boss-appeared-s2-2026'), ('yani-neko-tv-2026')
), inventory AS (
  SELECT works FROM episode_inventory_snapshots i
  WHERE checked_at > now() - interval '48 hours'
    AND NOT EXISTS (
      SELECT FROM targets t WHERE NOT EXISTS (
        SELECT FROM jsonb_array_elements(i.works) w WHERE w->>'slug'=t.slug AND w ? 'missing'
      )
    )
  ORDER BY checked_at DESC LIMIT 1
), queued AS (
  INSERT INTO episode_production_requests
    (id,world,first_episode,last_episode,token_limit,operator_paused,paused_reason,production_policy)
  SELECT md5('gz-20-aired-20261125/'||t.slug||'/'||n.number)::uuid,t.slug,n.number,n.number,
         100000,true,'等待广州批次',
         '{"character_consistency":"standard","preserve_published":true,"publish_to_reader":false,"queue":"gz-20-aired-20261125"}'
  FROM inventory i CROSS JOIN LATERAL jsonb_array_elements(i.works) w
  JOIN targets t ON t.slug=w->>'slug' JOIN work_worlds world ON world.slug=t.slug
  CROSS JOIN LATERAL jsonb_array_elements_text(w->'missing') e(value)
  CROSS JOIN LATERAL (SELECT e.value::int AS number) n
  WHERE NOT EXISTS (SELECT FROM episode_plans p WHERE p.world_slug=t.slug AND p.number=n.number)
    AND NOT EXISTS (SELECT FROM work_chapters c WHERE c.v=world.version_id AND c.episode=n.number)
    AND NOT EXISTS (SELECT FROM episode_production_requests r WHERE r.world=t.slug
      AND n.number BETWEEN r.first_episode AND r.last_episode
      AND r.stage NOT IN ('blocked','local_ready','published'))
  ON CONFLICT (id) DO NOTHING
  RETURNING id,revision
), messages AS (
  INSERT INTO episode_plan_messages(request_id,revision,instruction,author)
  SELECT id,revision,'核对该版本的官方分集与独立指南，复用已有角色资料，完成制作库五维验收。','ops:guangzhou-batch'
  FROM queued
)
SELECT count(*) AS enqueued FROM queued;

COMMIT;
