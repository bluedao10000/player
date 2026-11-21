-- Each policy is a lifetime production envelope for one edition. Changing its
-- range never refunds an earlier admission or alters an approved chapter.
CREATE TABLE episode_policies (
 world_slug text PRIMARY KEY REFERENCES work_worlds(slug),
 first_episode int NOT NULL CHECK(first_episode>0),
 last_episode int NOT NULL CHECK(last_episode BETWEEN first_episode AND first_episode+999),
 priority int NOT NULL CHECK(priority BETWEEN 0 AND 100),
 max_episodes int NOT NULL CHECK(max_episodes>=0),
 token_budget bigint NOT NULL CHECK(token_budget>=0),
 reserve_tokens int NOT NULL CHECK(reserve_tokens>0)
);
CREATE TABLE episode_admissions (
 topic text PRIMARY KEY REFERENCES episode_plans(topic),
 reserve_tokens int NOT NULL CHECK(reserve_tokens>0),
 admitted_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE episode_capture_attempts (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 topic text NOT NULL REFERENCES episode_plans(topic),
 started_at timestamptz NOT NULL DEFAULT now(),
 finished_at timestamptz,
 error text
);
CREATE INDEX episode_capture_attempts_topic ON episode_capture_attempts(topic,started_at DESC);
CREATE UNIQUE INDEX episode_capture_active ON episode_capture_attempts(topic) WHERE finished_at IS NULL;

CREATE VIEW episode_admission_usage AS
 SELECT a.topic,a.reserve_tokens,a.admitted_at,p.world_slug,p.number,
 coalesce((SELECT sum(m.total_tokens) FROM model_usage_receipts m WHERE m.receipt_rank=1
 AND m.subject IN (
  SELECT 'research:'||r.id FROM research_runs r WHERE r.topic=a.topic
  UNION ALL SELECT 'media:'||s.slot||':r'||s.round||':'||s.step||':a'||s.attempt
  FROM episode_art_jobs j JOIN media_candidates c ON c.art_job=j.id JOIN media_steps s ON s.slot=c.slot
  WHERE j.world_slug=p.world_slug AND (j.kind='scene' AND j.slug=p.spec->>'sceneSlug'
  OR j.kind='portrait' AND j.spec->>'slug' IN (SELECT x->>'slug' FROM jsonb_array_elements(p.spec->'people') x)
  AND s.started_at>=a.admitted_at AND NOT EXISTS (SELECT FROM episode_admissions older
    JOIN episode_plans op ON op.topic=older.topic WHERE op.world_slug=p.world_slug AND older.admitted_at<a.admitted_at
    AND op.spec->'people' @> jsonb_build_array(jsonb_build_object('slug',j.slug)))))
 ),0)::bigint AS measured_tokens
 FROM episode_admissions a JOIN episode_plans p USING(topic);

CREATE VIEW episode_policy_queue AS
 WITH inventory AS (SELECT checked_at,works FROM episode_inventory_snapshots ORDER BY checked_at DESC LIMIT 1),
 budgets AS (SELECT world_slug,count(*) AS admissions,sum(greatest(reserve_tokens,measured_tokens)) AS committed
 FROM episode_admission_usage GROUP BY world_slug)
 SELECT pol.*,n AS number,p.topic,a.admitted_at,a.measured_tokens,coalesce(b.committed,0) AS committed,
 h.published,cap.started_at AS capture_started,cap.finished_at AS capture_finished,cap.error AS capture_error,
 CASE WHEN h.published IS NOT NULL THEN '五维通过'
 WHEN a.topic IS NOT NULL THEN CASE WHEN cap.error IS NOT NULL THEN '采集失败' WHEN c.topic IS NULL THEN '待采集' ELSE '已采集' END
 WHEN i.checked_at IS NULL OR i.checked_at<now()-interval '48 hours' THEN '等待有效盘点'
 WHEN NOT EXISTS (SELECT FROM jsonb_array_elements(i.works) w WHERE w->>'slug'=pol.world_slug AND w->'missing' @> to_jsonb(ARRAY[n])) THEN '未确认缺集'
 WHEN p.topic IS NULL THEN '缺分集配置'
 WHEN coalesce(b.admissions,0)>=pol.max_episodes THEN '集数预算已满'
 WHEN coalesce(b.committed,0)+pol.reserve_tokens>pol.token_budget THEN 'token 预算不足'
 ELSE '待准入' END AS status
 FROM episode_policies pol CROSS JOIN LATERAL generate_series(pol.first_episode,pol.last_episode)n
 LEFT JOIN inventory i ON true LEFT JOIN budgets b USING(world_slug)
 LEFT JOIN episode_plans p ON p.world_slug=pol.world_slug AND p.number=n
 LEFT JOIN episode_admission_usage a ON a.topic=p.topic LEFT JOIN scope_heads h ON h.topic=p.topic
 LEFT JOIN episode_source_captures c ON c.topic=p.topic
 LEFT JOIN LATERAL (SELECT * FROM episode_capture_attempts e WHERE e.topic=p.topic ORDER BY started_at DESC LIMIT 1)cap ON true;
