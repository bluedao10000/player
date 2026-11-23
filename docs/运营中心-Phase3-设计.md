# 运营中心 Phase 3：全局运营拓扑设计

> 状态：评审通过，作为 Phase 3 实现依据。  
> 依据：运营中心重构审计第 1.2、3、4、5 节；Phase 0 设计第 1–4 节；Phase 1 设计。

## 1. 目标与边界

把 Phase 0 的 17 个运营阶段呈现为可筛选的全局拓扑。必须分别表达：

- **工作流定义**：阶段、上下游关系、职责和展示分组，来源为 ops_stages。
- **实际任务**：运行记录和当前待处理项，分别来源为 ops_task_runs、ops_work_items。
- **Agent 执行实例**：来源为工作进程心跳中的 executions；它表示进程当前执行情况，不等同于任务数。

tracked=false 的阶段仍作为工作流定义节点显示，标注「无独立执行记录」，不显示 0 或伪造任务指标。Phase 3 只做读模型和展示，不改变 Phase 0 视图语义、编辑任务、补集准入或发布保护状态机。

## 2. 现状与复用

### 2.1 WorkflowCanvas

现有 apps/web/src/features/workbench/workflow/workflow-canvas.tsx 将流程写成两组各 6 个本地节点：同步流程和媒体验收流程。latestRun 取最新一条记录，mediaStatus 用候选与步骤的 some 推断状态，nodeState 按任务标签筛选并把心跳不可用推为失败，还会只展示最近 5 条记录。它没有完整 17 阶段、真实阶段计数、窗口统计或作品筛选。

Phase 3 删除硬编码节点数组、基于候选推断节点状态的逻辑和“最近五条等于总量”的呈现。保留 board.css 的颜色主题与面板结构，复用 Panel、StatusBadge、节点图标和右侧检查器的视觉约定；新增样式由接口驱动的分组、依赖箭头、节点指标和作品进度使用。LiveTerminal 作为拓扑下方默认折叠的实时记录面板；其有限条数只用于浏览，不作为指标来源。

### 2.2 Phase 0 数据面

ops_stages 定义 stage、label、grp、upstream、tracked、position；ops_task_runs 提供执行状态、结果、作品归属、开始/结束时间、错误及来源引用；ops_work_items 提供当前 pending、blocked、waiting_human；ops_worker_heartbeats 提供 worker、role、loop、状态、期望心跳周期、最近心跳、错误和 executions。

工作流定义、任务数和执行实例分别聚合，不互相代替。Phase 1 的 overview、attention 继续保留原接口职责；拓扑使用相同 Phase 0 来源，并复用心跳在线判定。

## 3. 五组拓扑与基础设施栏

PRD 要求 5 组，Phase 0 的 grp 保留原有 6 种语义。迁移 062 新增 nullable topology_group，仅允许以下五种值；按 grp 为 16 个业务阶段赋组，backup 保持 grp=infrastructure、topology_group=NULL，在拓扑外单独显示「基础设施」栏。不得改变 grp 或 Phase 0 视图。

| PRD 展示组 | 阶段 ID | 阶段名 | Phase 0 grp | 上游 | tracked |
|---|---|---|---|---|---|
| Data Acquisition | source_sync | 来源同步 | acquisition | — | 是 |
| Knowledge Processing | normalize | 规范化 | knowledge | source_sync | 否 |
| Knowledge Processing | quality_scan | 质量扫描 | knowledge | normalize | 是 |
| Research & Planning | inventory | 目录盘点 | research | quality_scan | 否 |
| Research & Planning | plan | 分集计划 | research | inventory | 是 |
| Research & Planning | configuration | 制作配置 | research | plan | 是 |
| Research & Planning | admission | 补集准入 | research | configuration | 是 |
| Research & Planning | capture | 补集采集 | research | admission | 是 |
| Research & Planning | research | 资料取证 | research | capture | 是 |
| Content Production | editorial | 编辑制作 | production | — | 是 |
| Content Production | media_reference | 图片参考 | production | research | 是 |
| Content Production | media_generate | 图片生成 | production | media_reference | 是 |
| Content Production | media_review | 图片验图 | production | media_generate | 是 |
| Content Production | media_rework | 图片重做/复验 | production | media_review | 是 |
| Quality & Release | acceptance | 范围验收 | release | research、media_review | 是 |
| Quality & Release | release | 阅读器发布 | release | acceptance | 是 |

拓扑外基础设施栏只有 backup：grp=infrastructure、topology_group=NULL、无作品归属。五组节点和连线继续由 ops_stages 的定义、position 与 upstream 驱动；backup 也来自 ops_stages，但不塞进五组。

## 4. Agent loop 与阶段对应

### 4.1 建模建议

loop 与 stage 是多对多关系，迁移 062 新增 ops_stage_worker_loops(stage_id, role, loop) 映射表，主键为三列并以 ops_stages 外键约束。原因是一个 loop 可覆盖多个阶段、一个阶段也可能由多个 loop 执行；不宜把单个 loop 塞进 ops_stages 单列。Agent executions 的键是心跳 JSON 契约：拓扑查询按 stage id 读取 detail key，并在代码中兼容旧聚合 key；不新增执行键映射表，保证 062 只维护分组与 loop 对应关系。

| 阶段 | role / loop 映射 | Agent execution key | 展示说明 |
|---|---|---|---|
| source_sync | sync / source-sync | — | 调度工作进程 |
| normalize | — | — | 无独立执行记录 |
| quality_scan | sync / quality-scan | — | 调度工作进程 |
| inventory | — | — | 无独立执行记录 |
| plan | sync / episode-planner；sync / episode-plans；research-lane / research；agent / agent | planning | 计划生成与计划装载由调度 loop 承担 |
| configuration | sync / episode-planner；research-lane / research；agent / agent | configuration | 规划器和 research lane 都可进入配置步骤 |
| admission | research-lane / maintenance | — | 维护 loop 的准入处理 |
| capture | — | — | 一次性 CLI claim，不是常驻 loop |
| research | sync / research；research-lane / research；agent / agent | research | 资料取证 |
| editorial | sync / editorial；agent / agent | editorial | 调度任务与 Agent 执行实例分列 |
| media_reference | sync / media；media-lane / media；agent / agent | media_reference | 媒体 loop 加 Agent 当前步骤 |
| media_generate | sync / media；media-lane / media；agent / agent | media_generate | 同上 |
| media_review | sync / media；media-lane / media；agent / agent | media_review | 同上 |
| media_rework | sync / media；media-lane / media；agent / agent | media_rework | recrop 与 recheck 都归此阶段 |
| acceptance | sync / acceptance；media-lane / maintenance | — | 维护 loop 也会处理验收 |
| release | — | — | systemd timer 执行发布，没有阶段心跳 |
| backup | — | — | Kubernetes CronJob 执行，没有阶段心跳；单列于拓扑外「基础设施」

已核实的调度 loop 共 11 个：source-sync、quality-scan、episode-planner、episode-plans、research、acceptance、media、editorial、maintenance、usage、daily-snapshot；Agent 另有 agent loop。usage、daily-snapshot 不直接映射运营阶段。capture 由 refresh 定时脚本调用一次性 CLI claim；release 由 folio-reader-release.timer 调用 reader-release send；backup 由 Kubernetes CronJob 执行。因此这三阶段显示「无对应工作进程」，Agent 数不显示 0。

### 4.2 Agent 执行实例是否能细分

当前 Agent 汇总 executions 只有 media、research、editorial、configuration 四类，且 planning 和 configuration 请求目前都计入 configuration。仅凭当前四个计数不能拆出媒体细分阶段。

媒体任务 Harness 记录包含输入 step，且入口校验 step 为 reference、generate、review、recrop、recheck；因此可在 Agent 进程内只读检查非 terminal Harness 任务并分组：reference→media_reference，generate→media_generate，review→media_review，recrop/recheck→media_rework。该划分基于任务当前输入步骤，能可靠表示当前阶段，但不提供历史阶段执行次数。建议保留原 media 汇总字段以兼容 Phase 0/1，同时新增 media_reference、media_generate、media_review、media_rework 四个心跳计数。

planning 与 configuration 应新增独立计数键：将 planning 路由计入 planning，将 configuration 路由计入 configuration。work process 数量从 role/loop 心跳映射统计；执行实例数从 Agent 心跳 executions 的细分键统计，两者分开显示。

一个 worker 可能同时映射多个 stage。节点分别计数 distinct worker_id；组级汇总对 worker_id 去重，避免同一工作进程被重复相加。在线规则沿用 Phase 0：最近心跳年龄不大于 max(3 × expected_seconds, 120 秒) 为在线；有 last_error 但未过期显示在线且有错误。无映射显示「无对应工作进程」；有映射但没有在线进程显示数字 0。

## 5. 指标定义与时间边界

### 5.1 窗口

接口 window 取 24h、7d、30d，默认 24h，分别按滚动 24、168、720 小时计算。对一次请求固定 as_of=statement_timestamp()，窗口为 [as_of - duration, as_of)，所有边界均为 timestamptz 瞬时；界面和文档用 Asia/Shanghai（北京时间）展示。窗口统计用执行 finished_at 落在窗口内的终态记录，边界左闭右开。当前状态不受 window 影响。

### 5.2 节点卡指标

| 指标 | 来源与口径 |
|---|---|
| 当前任务总量 | Running + Pending + Blocked + Waiting Human；四项互斥。Running 只来自 ops_task_runs.status=running；其余来自 ops_work_items.state。 |
| Running | 当前 running 执行数；不受窗口影响。 |
| Pending | 当前 pending work item 数；不受窗口影响。 |
| Blocked | 当前 blocked work item 数；不受窗口影响。 |
| Waiting Human | 当前 waiting_human work item 数；不受窗口影响。 |
| Success / Failed | 终态执行按 ops_task_runs.status 分组；不使用 outcome 推导。 |
| 成功率 | success / (success + failed)；skipped 不进分母，分母为 0 时返回 null。 |
| 业务结果分布 | 另按 outcome 展示，例如图片通过/重绘/重裁、验收通过/未通过；不计入成功率。 |
| 在线工作进程 | 按 4.1 映射 role+loop 后，distinct worker_id 且符合 Phase 0 在线规则的数量。 |
| 运行中执行实例 | Agent 心跳 executions 对应细分 key 的当前非终态任务数；不等同于 task run 数。过期 Agent 心跳不作为当前值，应返回 null 和 freshness 状态。 |
| 执行耗时 | 窗口内有 started_at、finished_at 且 finished_at≥started_at 的终态执行；返回平均值、中位数、P90；running 不计。主数字采用中位数，界面明确标注「中位数」；平均值和 P90 同时显示。中位数减少重试和长尾对节点读数的干扰。 |
| 最近执行时间 | 全历史最近一次任务执行的 coalesce(finished_at, started_at)，不随 window 切换；无记录为 null。 |
| 吞吐量 | 窗口内终态执行数（success + failed + skipped）/完整窗口小时数，单位「次 / 小时」。不按首末记录跨度外推；0 条为 0/h，1–4 条仍给真实窗口速率并标「样本较少」，不作趋势结论。 |
| 无独立记录阶段 | tracked=false：只返回定义字段，metrics=null，显示「无独立执行记录」。 |

failure 原因在节点详情按 ops_task_runs.error 分组；当前阻塞原因来自 ops_work_items.reason。结果字段和失败原因不混合。

### 5.3 作品过滤与全库任务

选择 world_slug 后，所有作品级任务、队列和 Agent 任务关联按精确 world_slug 过滤。selected_world.has_records 用该作品全部历史任务或待处理项判断；没有任何记录的节点淡化显示。所选作品在某阶段有 blocked 或 waiting_human 时将节点标为阻塞位置。

Phase 0 中 source_sync、quality_scan、backup 的 world_slug 为空，属于全库任务：返回 scope=global 并标「全库任务」，不因选中作品而混入作品数字或生产进度。source_sync、quality_scan 保留在所属拓扑组；backup 单列在拓扑外「基础设施」栏。选择作品时基础设施栏仍显示「全库任务」，计数不受作品过滤。

## 6. 接口设计

均放在现有管理员分组 /api/v1/admin/knowledge/ops/ 下，沿用管理员权限和编辑会话要求。

### 6.1 GET /api/v1/admin/knowledge/ops/topology

参数：window=24h|7d|30d（默认 24h），world=<world_slug>（可选）。

返回字段：
- as_of、timezone、window、window_start、window_end、definition
- selected_world：slug、title、has_records、progress（未选时 null）
- groups：五个 topology_group 的 id、label、position、stages；infrastructure：拓扑外阶段列表
- stages：id、label、description、topology_group、position、tracked、upstream、global_scope、has_records、bottleneck
- current：total、running、pending、blocked、waiting_human
- window_stats：success、failed、skipped、success_rate、outcome_distribution、duration{average,median,p90,sample_count}、throughput_per_hour、sample_quality
- agents：mapped、online_workers、workers_with_error、running_executions、execution_coverage、legacy_unclassified、freshness、last_heartbeat_at
- 无数据或不适用的统计值为 null；不能用 0 代表“无映射”或“无独立记录”。

返回五组全部定义节点和独立 infrastructure 列表。tracked=false 的阶段保留定义但指标为 null。作品筛选时，全库阶段保留自身统计并通过 `global_scope=true` 标记「全库任务」，不混入选中作品的工作量。

### 6.2 GET /api/v1/admin/knowledge/ops/topology/stages/:stage

参数：window、world 同上；runs_after 与 queue_after 为分页游标；limit 默认 20、最大 100。

返回阶段定义、职责、拓扑分组、upstream/downstream、关联 worker（role、loop、在线状态、last_error、last_seen）、当前 Agent 执行计数、current_queue、窗口指标、outcome_distribution、failure_reasons、recent_runs 和 pagination。执行记录带 run_key、status、outcome、started_at、finished_at、duration_seconds、error、world_slug、episode 和来源定位字段；队列项使用来源视图的 item_key、state、reason、since、world_slug、episode 及 `link.page`/`link.params`。执行项同样返回可跳转 link。游标采用稳定的时间戳与主键组合，避免分页时重复或遗漏。

### 6.3 与 Phase 1 接口的关系

- /ops/overview 继续负责总览卡和全局关注摘要；不以 topology 替换。
- /ops/attention 继续提供跨阶段的完整关注列表；节点面板只查当前 stage 的队列和执行历史。
- topology 新增五组全局定义和节点聚合；stage detail 是按节点懒加载的队列、失败、Agent、执行明细。
- 三者共享 Phase 0 views 和在线判定，不复制业务写入逻辑。

Phase 1 overview.running_executions 与拓扑统一：只统计符合 Phase 0 在线判定的 Agent 心跳 executions；该指标 definition 升为 2。过期心跳不计入当前执行实例数。

## 7. 作品生产进度：六态

仅选中作品时计算；分母是该作品所有补集策略 episode_policies 的完整集号范围。先将每条策略的 [first_episode,last_episode] 展开，再按作品标识与集号去重；范围重叠只计一次。一个作品有多个 edition/world_slug 时，策略属于同一 work_id 的集号合并去重。

六态互斥，优先级为：未登记 > 已发布 > 已验收 > 生产中 > 已规划 > 已登记。策略范围内不存在对应 episode_plans 行的集数为「未登记」；存在 episode_plans 行的集数沿用原五态判定口径。五态信号沿用 episode_configuration_attempts、episode_admission_usage/admitted_at、scope_heads、scope_acceptances、reader_release_attempts；最高优先级状态覆盖较低状态。分母为去重后的策略集号数，六态合计必须等于分母。

接口返回六态数量、scope_total、as_of；不在 Phase 3 做趋势或日快照。episode_policy_queue 中的无配置项属于策略范围，计入「未登记」，同时仍可在补集准入节点的当前队列查看。

## 8. 性能估算与 062 索引

### 8.1 开发库只读盘点

查询对象是本机 tidepost-knowledge-postgres-1 容器内 PostgreSQL 17，角色 anime、库 anime_knowledge。仅执行只读事务、设置语句超时；数据库迁移仍在 057，无 ops views。以下计数来自真实表，不是结构推测：

| 来源表 | 当前行数 | 最近 30 天相关记录数 |
|---|---:|---:|
| works | 39,120 | 不用于拓扑聚合 |
| work_worlds | 25 | 不用于拓扑扫描 |
| knowledge_sync_runs | 124 | 124 |
| knowledge_quality_runs | 17 | 17 |
| episode_production_requests | 1 | 1 |
| episode_policy_queue | 0 | 0 |
| episode_plans | 61 | 不作为执行次数 |
| episode_configuration_attempts | 1 | 1 |
| episode_capture_attempts | 0 | 0 |
| research_runs | 85 | 85 |
| editor_tasks | 4 | 终态任务事件 0 |
| editor_events | 42 | 0 条终态事件 |
| media_candidates | 259 | 作为当前 work item 来源 |
| media_steps | 827 | 827 条有结束时间 |
| scope_acceptances | 282 | 282 |
| reader_release_attempts | 0 | 0 |
| backup_runs | 0 | 0 |

从来源执行记录估算，ops_task_runs 约 1,336 行，另有至多 4 个 editor task 投影；editor_events 需参与编辑任务事件关联。开发库现有样本都落在最近 30 天，但它只代表本机规模，不能外推生产规模。ops_task_runs 与 ops_work_items 视图在 057 不存在，因而无法在此库运行视图 EXPLAIN。

### 8.2 索引和迁移建议

迁移 062 只增加 topology_group 与 loop/stage 映射关系，不增加时间索引。现有时间索引与缺少索引的来源表盘点保留在上节，作为后续性能核验清单。

完成制作库行数盘点和 topology 查询的执行计划分析后，再针对确认的谓词单独创建索引。使用 PostgreSQL CREATE INDEX CONCURRENTLY，并在迁移事务之外执行，降低对运行中制作库的写入阻塞；不把索引放入当前事务式迁移 runner。拓扑查询先将 ops_task_runs、ops_work_items 各聚合一次，再按 stage 汇总，详情按需分页；30 天生产成本以实际 EXPLAIN (ANALYZE, BUFFERS) 和缓冲命中数据决定是否增索引或后续聚合缓存。不得改 Phase 0 视图语义。

## 9. 界面布局

- 桌面：过滤工具栏置顶，包含窗口切换（24 小时/7 天/30 天）和作品选择器；其下是作品六态进度条（有作品选择时显示）；主体按 5 组 swimlane 排列，组按 position 显示节点。依赖连接由 upstream 生成可点击的「上游 → 下游」关系条；拓扑旁独立放置「基础设施」栏。
- 节点卡：首行阶段名与状态；当前状态一行展示 Running、Pending、Blocked、等待人工、当前任务总量；窗口统计区展示 Success、Failed、成功率、吞吐量、最近执行时间和耗时；耗时主值明确标「中位数」，同时显示平均值和 P90；底部展示在线工作进程/运行中执行实例。结果分布和失败原因放入详情面板，避免卡片过载。
- 节点详情：点击后右侧检查器展示职责、Agent/worker、上下游、当前队列、窗口成功率、失败原因及最近执行记录；支持分页，不在初始页面为 17 个阶段同时取明细。
- 作品选择：突出作品经过的节点；无作品记录节点淡化，blocked 或 waiting_human 节点强调为阻塞位置；全库任务显示独立标记且不参与作品进度。基础设施栏在作品筛选时仍显示全库备份任务。
- LiveTerminal 保留在拓扑下方并默认折叠，供查看实时记录；不参与节点数字计算。
- 移动端：五组改为纵向可折叠分组，窗口与作品选择控件换行；依赖连接保持为文字箭头列表，节点详情改为页面下方检查器，上下游也以文字列表展示；LiveTerminal 默认折叠，带执行日志深链时自动展开。

## 10. 旧链接

Phase 1 的旧链接 /workbench?section=history 仍重定向到 /workbench/operations。本阶段保留运营拓扑路由，不改重定向和参数传递；回归测试继续断言 section=history 落在 /workbench/operations。

## 11. 迁移与测试计划

迁移从 062 开始。内容限于 ops_stages 的五组 topology_group 赋值和 ops_stage_worker_loops 对应表；backup 不设置 topology_group。时间索引不进入 062，待制作库行数和执行计划核验后在事务外单独 CREATE INDEX CONCURRENTLY。不得修改 Phase 0 视图已有列和语义。迁移空库、057 升级、重复执行和初始映射完整性均需覆盖。

隔离 PostgreSQL 集成测试至少覆盖：
1. 五组、17 阶段、所有 upstream 引用有效，关系可绘制且无意外环。
2. tracked=false 不产生 0 指标；Running 与 Pending 互斥；各状态映射及未知状态失败。
3. 24h/7d/30d 参数、北京时间展示、左闭右开边界、固定 as_of；窗口外记录不计入。
4. status 成功率和 outcome 分布分离；跳过状态、失败原因；running 不进入耗时；平均值、中位数、P90 样本正确。
5. 吞吐量固定以完整窗口小时为分母，0 和 1–4 条样本标记正确。
6. 在线阈值、last_error、无映射阶段、同 worker 多阶段去重；过期 Agent 心跳不冒充当前执行实例。
7. 媒体 step 到四个细分阶段、planning/configuration 分开计数；Agent 计数不与调度任务数相加。
8. 作品 world_slug 精确过滤、全库任务独立返回、空节点淡化、阻塞节点识别。
9. 六态互斥且总和等于所有补集策略的去重集号范围；无 episode_plans 行归未登记；同一作品多条重叠策略按集号去重。
10. 节点详情失败原因、游标稳定、limit 上限、来源深链；section=history 路由回归。
11. 在生产同量级数据上记录 30 天查询 EXPLAIN (ANALYZE, BUFFERS) 和缓冲命中情况，验证是否需要索引或后续聚合缓存。

## 12. 与审计稿的差异及已决定事项

### 差异

- 当前 WorkflowCanvas 是两组硬编码局部流程，设计改为 Phase 0 的 17 阶段；其中 16 个业务阶段进入 PRD 五组，backup 留在拓扑外基础设施栏。
- 审计稿 topology 接口建议有 world 参数；本稿增加 window 参数及节点详情分页接口。
- PRD 点名平均耗时，审计稿建议中位数与 P90；接口和界面同时显示平均值、中位数、P90，主数字为中位数。
- Phase 0 保留 infrastructure grp；迁移只为 16 个业务阶段配置 topology_group，backup 的 topology_group 为 NULL。
- Agent 原有四个执行计数过粗；媒体任务可由 Harness step 拆分当前阶段，planning/configuration 独立计数，兼容键保留。
- 不改变任何现有任务状态机、发布保护和 Phase 0 视图语义；Phase 1 running_executions 仅改变在线过滤定义并升版。

### 评审结论（第 12 节 6 项，均已决定）

1. **耗时主指标：已决定。**中位数为主数字，界面明确标注「中位数」；平均值与 P90 同时显示。
2. **作品进度：已决定。**改为六态；未登记排最前，策略范围内无 episode_plans 行即未登记。分母取所有 episode_policies 的完整集号范围，同一作品按集号去重。六态互斥且总数等于分母。
3. **备份分组：已决定。**backup 不进入五组，拓扑之外单独列「基础设施」；保留 grp=infrastructure，062 不赋 topology_group。作品筛选时仍显示为「全库任务」。
4. **Phase 1 在线执行数：已决定。**running_executions 只汇总在线 Agent 心跳，在线沿用 Phase 0 判定，definition 升为 2。
5. **Compose 节点名：已决定。**本地仅使用根 compose.yaml 入口，OPS_NODE_NAME 缺失或留空时默认 local，shell 和根 .env 的显式值优先，无需 compose.local-env；deploy/docker-compose.yml 保持变量必填，生产必须显式填写每节点唯一值。production lane.env 继续逐节点显式配置；两份 env 示例说明本地默认和生产唯一要求。
6. **迁移与时间索引：已决定。**062 只包含五组拓扑分组和 loop/stage 对应关系，不加索引。先在制作库核对行数与执行计划，再在迁移事务外用 CREATE INDEX CONCURRENTLY 创建经确认的索引。
