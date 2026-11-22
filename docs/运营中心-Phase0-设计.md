# 运营中心 Phase 0：数据基础设计

实施稿，2026-11-22。接[重构审计](运营中心重构审计.md)第 6、7 节。已确认的决定：「Agent 数」拆成「在线工作进程」和「运行中执行实例」两项展示；各节点 Pi Durable 的执行状态通过心跳上报（方案 A）。本文记录 Phase 0 的数据口径和部署结论。

## 范围

| 做 | 不做 |
| --- | --- |
| 1 张阶段配置表、2 个只读视图、1 张心跳表、1 张日快照表、1 张旧图片批次归属表 | 不改任何现有业务表的写入路径和状态机 |
| 调度循环与 Agent 上报心跳；每日写一次快照 | 不做新界面；工作台只在阶段 1 接入 |
| 隔离 PostgreSQL 测试覆盖每个来源、每种状态 | 不建异常台账、决策记录、完整度规则（阶段 2、5） |

## 对审计稿的两处更正

- 编辑任务 worker（`ProcessEditorial`）运行在 `apps/sync` 的调度进程里，不在 Go API 里。见[同步调度](../apps/sync/internal/service/scheduler/sync.go)。
- Agent 不是每个节点只有一份 Pi Durable 存储。图片任务用进程级 Harness（`agent.sqlite`）；取证和编辑每次运行各开一份独立存储（`research-<run>.sqlite`、`<name>.sqlite`），在一次 HTTP 请求内跑完。见[取证](../apps/agent/src/research.ts)、[编辑](../apps/agent/src/editor.ts)。这决定了下文心跳里执行数的统计方式。

## 三层模型

PRD 要求区分「工作流定义」「实际任务数量」「Agent 执行实例」。Phase 0 分别用三个对象表达，界面不能互相替代：

| 层 | 对象 | 回答的问题 |
| --- | --- | --- |
| 工作流定义 | `ops_stages`（配置表） | 流程有哪些阶段、上下游是什么 |
| 实际任务 | `ops_task_runs`（执行记录视图）与 `ops_work_items`（待处理视图） | 每个阶段执行了多少次、成败如何；现在有多少在等、多少卡住 |
| 执行实例 | `ops_worker_heartbeats`（心跳表） | 哪些进程在线、各自正在执行几个任务 |

拓扑节点数等于 `ops_stages` 的行数；节点上的数字全部来自两个视图。定义了但没有独立记录的阶段（例如数据清洗、作品归并，它们在来源同步的同一事务里完成）显示为「无独立执行记录」，不显示 0。

## 1. 阶段定义 `ops_stages`

```sql
CREATE TABLE ops_stages (
  id text PRIMARY KEY,                  -- 与视图的 stage 列对应
  grp text NOT NULL CHECK (grp IN ('acquisition','knowledge','research','production','release','infrastructure')),
  position int NOT NULL,
  title text NOT NULL,                  -- 中文主名
  eyebrow text NOT NULL,                -- 英文小标题
  upstream text[] NOT NULL DEFAULT '{}',
  tracked boolean NOT NULL DEFAULT true,-- false：只有定义，没有独立执行记录
  UNIQUE (grp, position)
);
```

初始行（`tracked=false` 的阶段在拓扑里只画定义）：

| grp | id | title | upstream | tracked |
| --- | --- | --- | --- | --- |
| acquisition | `source_sync` | 来源同步 | — | 是 |
| knowledge | `normalize` | 清洗与归并 | source_sync | 否 |
| knowledge | `quality_scan` | 知识质量扫描 | normalize | 是 |
| research | `inventory` | 分集盘点 | quality_scan | 否（盘点是快照，不是任务） |
| research | `plan` | 分集规划 | inventory | 是 |
| research | `configuration` | 制作配置 | plan | 是 |
| research | `admission` | 补集准入 | configuration | 是（只有待处理项） |
| research | `capture` | 原文采集 | admission | 是 |
| research | `research` | 取证与导读 | capture | 是 |
| production | `editorial` | 编辑修订 | — | 是 |
| production | `media_reference` | 参考图准备 | research | 是 |
| production | `media_generate` | 原图生成 | media_reference | 是 |
| production | `media_review` | 五项验图 | media_generate | 是 |
| production | `media_rework` | 重裁与复验 | media_review | 是 |
| release | `acceptance` | 五维验收 | research, media_review | 是 |
| release | `release` | 正式站发布 | acceptance | 是 |
| infrastructure | `backup` | 制作库备份 | — | 是 |

PRD 里的「正文生成」由取证阶段的 `write` 步骤完成，归入 `research`；编辑修订是人工发起的改稿，没有上游，单独成节点。

## 2. 执行记录视图 `ops_task_runs`

每行是一次执行尝试。只收录已经开始的执行，等待中的工作在 `ops_work_items`。

| 列 | 说明 |
| --- | --- |
| `run_key` | 全局唯一：`<来源>:<主键>`，例如 `media:12:r1:review:a2` |
| `stage` | 对应 `ops_stages.id` |
| `world_slug`、`episode_first`、`episode_last` | 归属作品与集号范围；无法归属时为 NULL |
| `subject` | 人能读懂的对象：来源名、章节 slug、场景名 · 候选号 |
| `attempt` | 该对象的第几次尝试 |
| `status` | `running`、`success`、`failed`、`skipped` 四值之一 |
| `outcome` | 执行成功时的业务结果：验图 `pass` / `redraw` / `recrop`，验收 `passed` / `rejected`；其他为 NULL |
| `started_at`、`finished_at` | 原表时间；瞬时事件两者相同 |
| `error` | 原表错误文本 |
| `source_table`、`source_id` | 回到原记录 |

`status` 只描述执行本身是否完成，业务上是否合格放在 `outcome`。例：验图模型正常返回「需要重裁」，`status=success`、`outcome=recrop`。这样「任务失败率」和「验图通过率」不会互相污染。

### 状态映射

| 来源表 | stage | 映射规则 | 作品归属 |
| --- | --- | --- | --- |
| `knowledge_sync_runs` | source_sync | `pending`→不收录（尚未开始）；`running`→running；`success` 且 `skipped`→skipped（来源版本未变）；`success`→success；`failed`→failed | 无（全库） |
| `knowledge_quality_runs` | quality_scan | 原值 | 无 |
| `episode_production_requests` 的规划过程 | plan | 本表没有逐次执行记录，Phase 0 不收录；规划卡住的情况在 `ops_work_items` 体现 | — |
| `episode_configuration_attempts` | configuration | `finished_at` 空→running；`error` 非空→failed；`registered`→success；已结束、无错误、未登记→skipped（防御性映射，见下文） | `episode_production_requests.world` |
| `episode_capture_attempts` | capture | `finished_at` 空→running；`error` 非空→failed；否则 success | 经 `episode_plans.topic` |
| `research_runs` | research | `collect` / `write`→running；`ready`→success；`failed`→failed | `research_topics.world_slug`，集号取首末集 |
| `editor_tasks` | editorial | `evidence` / `drafting` / `checking` 且持有 `lease`→running；`published`→success；`failed`→failed；其余不收录（在待处理视图） | `world`，subject 为章节 |
| `media_steps` | `reference`→media_reference；`generate`→media_generate；`review`→media_review；`recrop`、`recheck`→media_rework | `finished_at` 空→running；`error` 非空→failed；否则 success，`outcome` 取 `result->>'decision'` | `media_candidates.art_job`→`episode_art_jobs.world_slug`；旧批次查下方归属表 |
| `scope_acceptances` | acceptance | 瞬时事件，`started_at = finished_at = checked_at`；`status=success`，`outcome` 由 `passed` 决定 | `edition_slug`，`scope_first`–`scope_last` |
| `reader_release_attempts` | release | `finished_at` 空→running；`error` 非空→failed；`target_id` 非空→success | 经 `episode_plans.topic` |
| `backup_runs` | backup | 原值 | 无 |

**旧图片批次归属表。** `mio`、`bebop`、`bebop-portrait`、`bebop-e3` 等早期批次没有 `art_job`，作品归属写在 Agent 代码里。新增 `ops_media_batch_worlds(batch text PRIMARY KEY, world_slug text NOT NULL REFERENCES work_worlds(slug))`。迁移按当时 `media_candidates` 中实际存在的值登记：`mio`→`tidepost`，所有 `bebop%`→`cowboy-bebop-tv`；新增的匹配批次也会被迁移纳入。为通过空库迁移，只有目标作品已在 `work_worlds` 中时才写入映射；世界目录导入和分集登记新建作品时会在同一事务中调用幂等种子函数，自动补齐此前因目标作品缺失而未能创建的映射。视图不写 CASE 常量。

**制作配置的未登记结果。** `prepareConfiguration` 只有在 `RegisterEpisodeConfiguration` 成功后才以 `registered=true` 结束尝试；缺少可复用来源配置会写入错误文本。正常写入路径不会产生「已结束、无错误、未登记」。视图仍将这类手工或历史异常记录映射为 `skipped`，`outcome='unregistered'`，避免把它误计为成功。

**长时间运行中。** 视图不判断超时。`running` 行带 `started_at`，「运行超过 N 分钟」由阶段 5 的异常规则按阶段阈值判断。

## 3. 待处理视图 `ops_work_items`

每行是一件当前还没完成、也没有执行在跑的工作，只反映现在的状态，不保留历史。

| 列 | 说明 |
| --- | --- |
| `item_key`、`stage`、`world_slug`、`episode`、`subject` | 同上 |
| `state` | `pending`（等待自动执行）、`blocked`（前置条件不满足，自动流程推进不了）、`waiting_human`（等待人工批准） |
| `reason` | 机器可读原因码，界面再翻译成中文 |
| `since` | 进入该状态的时间；原表没有记录时为 NULL，界面显示「—」，不用当前时间顶替 |

### 判定规则

| 来源 | 条件 | state | reason |
| --- | --- | --- | --- |
| `episode_policy_queue` | 状态「待准入」 | pending | `awaiting_admission` |
| 同上 | 「待采集」 | pending | `awaiting_capture` |
| 同上 | 「已采集」且该 topic 没有进行中的取证 | pending | `awaiting_research` |
| 同上 | 「token 预算不足」 | blocked | `token_budget` |
| 同上 | 「集数预算已满」 | blocked | `episode_budget` |
| 同上 | 「等待有效盘点」 | blocked | `stale_inventory` |
| 同上 | 「缺分集配置」 | blocked | `missing_configuration` |
| 同上 | 「采集失败」 | pending | `capture_retry` |
| 同上 | 「五维通过」「未确认缺集」 | 不收录 | — |
| `episode_production_requests` | `paused_reason` 非空 | blocked | 原值，例如 `insufficient_remaining_tokens`、`missing_usage_receipt` |
| 同上 | `stage='blocked'` | blocked | `plan_blocked` |
| 同上 | `stage` 为 `inventory`、`planning`、`drafting_plan` | pending | `planning` |
| `editor_tasks` | `evidence` / `drafting` / `checking` 且无 `lease` | pending | `awaiting_worker` |
| 同上 | `review` | waiting_human | `awaiting_review` |
| 同上 | `conflict` | blocked | `draft_conflict` |
| `media_candidates` | `queued`、`reference` 且无进行中步骤 | pending | `awaiting_media` |
| 同上 | `held` | waiting_human | `media_held` |

采集调度的 `ClaimCaptureLease` 会排除最近 15 分钟内结束的失败尝试，冷却期过后会再次领取，因此采集失败属于自动重试中的 pending 项。活动中的采集 lease 会让「待采集」项暂不进入待处理视图，避免与 running 执行重复统计。

`episode_policy_queue` 的 `status` 是用于界面显示的中文字面量；Phase 1 迁移 `060.sql` 在视图末尾追加 `status_code`，所有逻辑判断改用稳定状态码，并测试未知状态码时失败。

### Overview 指标与三层的对应

| 指标 | 计算 |
| --- | --- |
| 当前执行任务数 | `ops_task_runs` 中 `status='running'` |
| 等待队列任务数 | `ops_work_items` 中 `state='pending'` |
| 当前阻塞任务数 | `ops_work_items` 中 `state='blocked'` |
| 等待人工决策 | `ops_work_items` 中 `state='waiting_human'`（阶段 5 并入异常中心） |
| 今日自动完成 / 失败 | `ops_task_runs` 中 `finished_at` 落在北京时间今日，`status` 为 success / failed |
| 在线工作进程 | 心跳表中判定为在线的进程数 |
| 运行中执行实例 | 心跳上报的执行数之和，与「当前执行任务数」并列展示，两者不一致即为对账信号 |

## 4. 心跳 `ops_worker_heartbeats`

```sql
CREATE TABLE ops_worker_heartbeats (
  worker_id text NOT NULL,            -- '<节点>/<角色>'，例如 'gz/research-lane'
  loop text NOT NULL,                 -- 循环名，例如 'research'、'media'、'usage'
  role text NOT NULL CHECK (role IN ('sync','research-lane','media-lane','agent')),
  node text NOT NULL,
  version text NOT NULL,              -- 构建版本
  expected_seconds int NOT NULL CHECK (expected_seconds > 0),
  last_tick_at timestamptz NOT NULL,
  last_ok_at timestamptz,
  last_error text,
  last_error_at timestamptz,
  consecutive_errors int NOT NULL DEFAULT 0,
  executions jsonb,                   -- 仅 agent 行：各类任务的运行中数量
  PRIMARY KEY (worker_id, loop)
);
```

**Go 调度循环。** 新增一个小函数，在每个循环每轮结束时调用：成功则刷新 `last_tick_at`、`last_ok_at` 并把 `consecutive_errors` 清零；失败则记录错误文本（截断到 500 字）并累加。为控制写入量，同一循环状态没变化时最多 30 秒写一次。现在只写日志的循环全部接入：

| 进程 | 循环 | 现有间隔 |
| --- | --- | --- |
| `sync`（[Run](../apps/sync/internal/service/scheduler/sync.go)） | editorial、episode-planner、usage、episode-plans、research、acceptance、来源同步、质量扫描 | 2 秒 – 1 分钟 |
| `research-lane` / `media-lane`（[RunLane](../apps/sync/internal/service/scheduler/production.go)） | maintenance、research 或 media、usage | 5 秒 / 1 分钟 |

`expected_seconds` 取循环间隔，`worker_id` 中的节点名来自 `OPS_NODE_NAME`，未配置或格式不合法时长期调度启动失败，避免两个节点写成同一行。检查过 `deploy/` 与 Compose 配置，原先没有可复用的节点名变量；单机示例值为 `local`，广州/硅谷分别使用 `gz` / `sv`。`APP_VERSION` 提供 Agent 构建版本，未设置时记录 `dev`。

**Agent。** Agent 进程没有数据库连接，也不应为此新增连接。做法是：

1. Agent 新增 `GET /internal/status`，返回版本、启动时间和 `executions`：
   - 图片任务：在进程级 Harness 上调用 Pi Durable `Harness.inspect()` 只读扫描，统计非 `terminal` 的 `tidepost.media-candidate` 任务。
   - 取证、编辑、制作配置：这三类在单次 HTTP 请求内执行，用进程内计数器统计正在处理的请求数。
2. 所在节点的车道调度每 30 秒调一次这个接口，以 `<节点>/agent` 写入心跳行。调用失败时只更新 `last_error`，不刷新 `last_ok_at`。

已核实 Pi Durable 1.5.2 提供只读的 `Harness.inspect()`，不需要开写事务。Agent 的版本来自 `APP_VERSION`，启动时间为进程启动时刻；取证、编辑、制作配置通过请求计数器统计。

**在线判定。** `last_ok_at` 在 `max(3 × expected_seconds, 120 秒)` 内为在线；超过为离线；`consecutive_errors ≥ 3` 时为「在线但持续出错」。规则写在查询里，不存状态列。

**权限。** 硅谷媒体节点使用受限数据库角色，需要在[媒体授权](../apps/sync/ops/production/media-grants.psql)里为心跳表加 `INSERT, UPDATE`；API 只读角色加 `SELECT`。

## 5. 日快照 `ops_daily_snapshots`

```sql
CREATE TABLE ops_daily_snapshots (
  day date NOT NULL,                   -- 北京时间日期
  scope text NOT NULL,                 -- 'all' 或作品 slug
  definition int NOT NULL,             -- 指标口径版本，口径变化时加 1
  metrics jsonb NOT NULL,
  taken_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (day, scope)
);
```

- **内容（definition 1）。** 全库：活跃作品数、按 `works.type` 原值分组的作品数（NULL 单列）、已发布作品世界数，以及 `reader_knowledge_inventory` 各项合计（人物、立绘、章节、章节配图、故事线、场景、地点）。逐作品：上述知识计数，以及已登记集数（`episode_plans`）、范围验收已通过集数、正式站已发布集数。完整度结果在阶段 2 加入，届时 `definition` 升为 2。
- **写入。** 单机/默认 Compose 部署由 `sync` 进程负责；分节点部署由 `research-lane` 负责，`media-lane` 不运行快照任务。进程在北京时间每天 00:05 之后的第一轮执行，一个事务写入当天全库行和逐作品行，`ON CONFLICT DO NOTHING`。重复执行、多进程同时执行都只留下第一份。
- **不补历史。** 调度停了一整天，那天就没有快照，趋势图上留空。不允许事后用当前数据回填，这是 PRD 第 7.4 节的硬性要求。
- **口径变化。** 新口径从新日期开始写，趋势图按 `definition` 分段展示，不重算旧快照。

## 6. 迁移与上线顺序

一个迁移 `059.sql`，只新增对象，不改现有表：

1. `ops_stages` 及初始行；`ops_media_batch_worlds` 及初始行。
2. `ops_task_runs`、`ops_work_items` 两个视图。
3. `ops_worker_heartbeats`、`ops_daily_snapshots`。
4. 媒体节点角色授权（随部署脚本执行，不放进迁移）。

上线顺序：先迁移，再发布 `sync` 与车道调度（开始写心跳和快照），最后发布 Agent 的状态接口。Agent 先于调度发布也没关系：新接口没人调用；调度先于 Agent 发布时，agent 行会显示调用失败，这正是心跳要暴露的情况。

Phase 0 不新增对外接口。验收用下方 SQL；阶段 1 再加 `ops/overview` 聚合接口。

## 7. 测试

按[开发规范](开发规范.md)的隔离 PostgreSQL 方式，fixture 放在 `apps/api/internal/testsupport/fixtures`：

- **映射。** 每张来源表的每个状态值各插一行，断言视图输出的 `stage`、`status`、`outcome`、作品归属。来源表的 CHECK 约束新增状态值而视图没跟上时，测试失败。
- **互斥。** 同一工作不能同时出现在 `ops_task_runs` 的 running 和 `ops_work_items` 里（例如已采集且取证进行中）。
- **队列字面量。** `episode_policy_queue` 出现未映射的状态时失败。
- **心跳。** 连续失败计数、成功清零、30 秒节流、在线判定的三档边界。
- **快照。** 同一天执行两次只有一份；跨北京时间零点时日期正确。
- **迁移。** 空库、现有数据、重复执行三种情况。

## 8. 验收查询

```sql
-- 各阶段执行量与成败
SELECT stage, status, count(*) FROM ops_task_runs GROUP BY 1, 2 ORDER BY 1, 2;
-- 待处理与阻塞原因
SELECT stage, state, reason, count(*) FROM ops_work_items GROUP BY 1, 2, 3 ORDER BY 4 DESC;
-- 无法归属作品的执行（应只有全库任务）
SELECT stage, count(*) FROM ops_task_runs WHERE world_slug IS NULL GROUP BY 1;
-- 工作进程在线情况
SELECT worker_id, loop, now() - last_ok_at AS silent, consecutive_errors, last_error
FROM ops_worker_heartbeats ORDER BY silent DESC NULLS FIRST;
-- 今日快照
SELECT scope, definition, metrics FROM ops_daily_snapshots WHERE day = (now() AT TIME ZONE 'Asia/Shanghai')::date;
```

## 已确认的实现结论

1. 制作配置正常写入路径不会产生「已结束、无错误、未登记」；若手工或历史数据出现，视图按 `skipped / unregistered` 处理。
2. 采集失败 15 分钟冷却后会重新领取，映射为 `pending / capture_retry`。
3. 图片候选 `held` 不会自动进入下一步，映射为 `waiting_human / media_held`。
4. 部署配置原先没有节点名变量，新增 `OPS_NODE_NAME`；值用 `gz`、`sv`、`local` 等稳定短名。Pi Agent 版本使用 `APP_VERSION`。
5. 默认 `sync` 进程负责快照；分节点时只有 `research-lane` 负责。

媒体心跳写权限已加入[媒体授权脚本](../apps/sync/ops/production/media-grants.psql)。仓库里没有单独的 API 只读角色及其授权文件；现有只读身份是 `folio_backup_read`，已在[源库只读授权](../deploy/pg-backup/source-grants.psql)中添加对运营中心对象的 `SELECT`。
