-- psql -v world=takopi-ona-2025 -f work-usage.sql
-- Every research attempt belongs to an episode; a reused portrait belongs to the work.
WITH subjects AS (
  SELECT 'research:' || r.id AS subject, '第 ' || p.number || ' 集' AS scope
  FROM episode_plans p JOIN research_runs r ON r.topic = p.topic
  WHERE p.world_slug = :'world'
  UNION ALL
  SELECT 'media:' || c.slot || ':r' || s.round || ':' || s.step || ':a' || s.attempt,
         CASE WHEN a.kind = 'portrait' THEN '共享肖像'
              ELSE '第 ' || (a.spec->>'episode') || ' 集' END
  FROM episode_art_jobs a JOIN media_candidates c ON c.art_job = a.id
  JOIN media_steps s ON s.slot = c.slot WHERE a.world_slug = :'world'
)
SELECT coalesce(scope, '全作') AS scope, purpose, provider_account, requested_model,
       count(*) AS receipts,
       count(*) FILTER (WHERE meter_state <> 'measured') AS needs_review,
       sum(input_tokens) FILTER (WHERE meter_state = 'measured') AS input_tokens,
       sum(coalesce((usage #>> '{prompt_tokens_details,cached_tokens}')::bigint,
                    (usage #>> '{input_tokens_details,cached_tokens}')::bigint))
           FILTER (WHERE meter_state = 'measured') AS cached_input_tokens,
       sum(output_tokens) FILTER (WHERE meter_state = 'measured') AS output_tokens,
       sum(total_tokens) FILTER (WHERE meter_state = 'measured') AS total_tokens,
       sum(images) AS images
FROM subjects JOIN model_usage_receipts USING (subject)
WHERE receipt_rank = 1
GROUP BY GROUPING SETS ((scope, purpose, provider_account, requested_model),
                        (purpose, provider_account, requested_model))
ORDER BY grouping(scope), scope, purpose;
