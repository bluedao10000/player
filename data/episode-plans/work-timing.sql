-- psql -v world=look-back-film-2024 -f work-timing.sql
WITH research AS (
  SELECT r.* FROM research_runs r JOIN episode_plans p ON p.topic = r.topic
  WHERE p.world_slug = :'world'
), media AS (
  SELECT c.slot, s.round, s.step, s.attempt, s.started_at, s.finished_at
  FROM episode_art_jobs a JOIN media_candidates c ON c.art_job = a.id
  JOIN media_steps s ON s.slot = c.slot WHERE a.world_slug = :'world'
), subjects AS (
  SELECT 'research:' || id AS subject FROM research
  UNION ALL
  SELECT 'media:' || slot || ':r' || round || ':' || step || ':a' || attempt FROM media
), calls AS (
  SELECT m.* FROM subjects JOIN model_usage_receipts m USING (subject)
  WHERE receipt_rank = 1
)
SELECT purpose, operation, requested_model, count(*) AS calls,
       sum(total_tokens) FILTER (WHERE meter_state = 'measured') AS tokens,
       sum(images) AS images, round(sum(extract(epoch FROM finished_at - started_at)), 1) AS call_seconds,
       count(*) FILTER (WHERE meter_state <> 'measured') AS unmeasured
FROM calls GROUP BY 1, 2, 3 ORDER BY 1, 2, 3;

SELECT p.number, r.started_at AS research_started, r.finished_at AS research_finished,
       a.checked_at AS approved_at
FROM episode_plans p
LEFT JOIN scope_heads h ON h.topic = p.topic
LEFT JOIN scope_acceptances a ON a.id = h.published
LEFT JOIN research_runs r ON r.id = a.research_run
WHERE p.world_slug = :'world' ORDER BY p.number;

SELECT s.step, count(*) AS attempts,
       count(*) FILTER (WHERE s.error IS NOT NULL) AS failed,
       round(sum(extract(epoch FROM s.finished_at - s.started_at)), 1) AS task_seconds,
       round(extract(epoch FROM max(s.finished_at) - min(s.started_at)), 1) AS elapsed_seconds
FROM episode_art_jobs a JOIN media_candidates c ON c.art_job = a.id
JOIN media_steps s ON s.slot = c.slot
WHERE a.world_slug = :'world'
GROUP BY s.step ORDER BY min(s.started_at);
