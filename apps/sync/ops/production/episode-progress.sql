BEGIN READ ONLY;

WITH targets(slug) AS (VALUES
  ('apothecary-diaries-s3-2026'), ('dark-summoner-on-air-2026'), ('enchanter-second-life-tv-2026'),
  ('fx-fighter-kurumi-tv-2026'), ('horemajo-tv-2026'), ('magical-explorer-tv-2026'),
  ('mushoku-tensei-s3-2026'), ('nia-liston-tv-2026'), ('ramparts-of-ice-s2-2026'),
  ('ranma-half-s3-2026'), ('reborn-as-space-mercenary-tv-2026'), ('reincarnated-as-a-sword-s2-2026'),
  ('romelia-war-chronicle-tv-2026'), ('seitokai-ana-tv-2026'), ('sekamajo-tv-2026'),
  ('smoking-behind-the-supermarket-tv-2026'), ('snow-widow-cursed-ring-tv-2026'), ('toaru-item-tv-2026'),
  ('wild-last-boss-appeared-s2-2026'), ('yani-neko-tv-2026')
), inventory AS (
  SELECT checked_at, works
  FROM episode_inventory_snapshots
  ORDER BY checked_at DESC LIMIT 1
)
SELECT checked_at AT TIME ZONE 'Asia/Shanghai' AS checked_cst,
       t.slug AS work,
       w.episodes AS registered_total,
       work->>'catalogEpisodes' AS catalog_total,
       work->>'published' AS reader_chapters,
       coalesce(work->>'observationSource','public') AS observed_via,
       (SELECT count(*) FROM episode_plans p JOIN scope_heads h ON h.topic = p.topic
        JOIN scope_acceptances a ON a.id = h.latest
        WHERE p.world_slug = t.slug AND a.passed AND p.spec @> '{"publishToReader":false}') AS local_ready,
       jsonb_array_length(work->'missing') AS aired_missing_on_site,
       jsonb_array_length(work->'today') AS today,
       jsonb_array_length(work->'future') AS future,
       jsonb_array_length(work->'undated') AS undated,
       left(work->>'note', 180) AS note
FROM targets t
LEFT JOIN work_worlds world ON world.slug = t.slug
LEFT JOIN works w ON w.id = world.work_id
LEFT JOIN inventory ON true
LEFT JOIN LATERAL jsonb_array_elements(inventory.works) work ON work->>'slug' = t.slug
ORDER BY t.slug;

WITH inventory AS (
 SELECT checked_at,works FROM episode_inventory_snapshots ORDER BY checked_at DESC LIMIT 1
), targets AS (
 SELECT DISTINCT world FROM episode_production_requests WHERE production_policy->>'queue'='gz-20-aired-20261125'
), coverage AS (
 SELECT checked_at,w->>'slug' AS world,(w->>'published')::int AS reader_chapters,
        jsonb_array_length(w->'missing') AS aired_gap,
        (SELECT count(*) FROM jsonb_array_elements_text(w->'missing') n
         JOIN episode_plans p ON p.world_slug=t.world AND p.number=n::int
         JOIN scope_heads h ON h.topic=p.topic JOIN scope_acceptances a ON a.id=h.latest AND a.passed) AS local_passed
 FROM inventory CROSS JOIN LATERAL jsonb_array_elements(works) w JOIN targets t ON t.world=w->>'slug'
 WHERE w ? 'missing'
)
SELECT max(checked_at) AT TIME ZONE 'Asia/Shanghai' AS catalog_cst, count(*) AS works_with_schedule,
       sum(reader_chapters) AS reader_chapters, sum(local_passed) AS local_passed_awaiting_release,
       sum(reader_chapters+aired_gap) AS aired_total, sum(aired_gap-local_passed) AS remaining,
       count(*) FILTER(WHERE aired_gap=local_passed) AS works_complete
FROM coverage;

SELECT r.world, r.first_episode, r.last_episode, r.stage, r.consumed AS tokens,
       count(*) FILTER (WHERE a.passed) AS accepted,
       coalesce(r.production_policy->>'recovery_result',r.paused_reason, attempt.error) AS last_recovery_or_blocker,
       attempt.error AS configuration_error
FROM episode_production_requests r
LEFT JOIN episode_request_specs s ON s.request_id = r.id
LEFT JOIN episode_plans p ON p.world_slug = s.world_slug AND p.number = s.number
LEFT JOIN scope_heads h ON h.topic = p.topic
LEFT JOIN scope_acceptances a ON a.id = h.latest
LEFT JOIN LATERAL (
  SELECT error FROM episode_configuration_attempts
  WHERE request_id = r.id
  ORDER BY started_at DESC LIMIT 1
) attempt ON true
WHERE r.production_policy->>'queue'='gz-20-aired-20261125'
GROUP BY r.id, attempt.error
ORDER BY r.created_at DESC;

SELECT r.world, count(*) AS queued,
       count(*) FILTER (WHERE a.passed) AS five_dimensions_passed,
       count(*) FILTER (WHERE r.stage='published') AS published,
       max(a.checked_at) AT TIME ZONE 'Asia/Shanghai' AS last_acceptance_cst
FROM episode_production_requests r
LEFT JOIN episode_plans p ON p.world_slug=r.world AND p.number=r.first_episode
LEFT JOIN scope_heads h ON h.topic=p.topic
LEFT JOIN scope_acceptances a ON a.id=h.latest
WHERE r.production_policy->>'queue'='gz-20-aired-20261125'
GROUP BY r.world ORDER BY r.world;

SELECT stage, paused_reason, count(*) AS episodes
FROM episode_production_requests
WHERE production_policy->>'queue'='gz-20-aired-20261125'
GROUP BY stage, paused_reason
ORDER BY stage, paused_reason;

SELECT world_slug, status, count(*) AS episodes, sum(measured_tokens) AS tokens
FROM episode_policy_queue
GROUP BY world_slug, status
ORDER BY world_slug, status;

COMMIT;
