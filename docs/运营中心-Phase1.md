# 运营中心 Phase 1：信息架构

实施稿，2026-11-22。依据[重构审计](运营中心重构审计.md)第 3–5、7 节与[Phase 0 设计](运营中心-Phase0-设计.md)第 3 节。Phase 0 已在 `9fd006e` 推送。本阶段保留本机访问、编辑会话和 `board.css` 主题；移动端提供页面查看。

## 路由表

| 路由 | 中文主名 / eyebrow | 页面内容与数据来源 |
| --- | --- | --- |
| `/workbench` | 运营总览 / OVERVIEW | Overview 聚合接口：指标卡、作品类型分布、前 20 条需要关注事项 |
| `/workbench/operations` | 运营拓扑 / OPERATIONS | 现有 `WorkflowCanvas` 与 `LiveTerminal`；拓扑计数留到 Phase 3 |
| `/workbench/knowledge` | 知识资产 / KNOWLEDGE | `WorldInventory`、`ScopeReport`、`MediaGallery` |
| `/workbench/projects` | 项目 / PROJECTS | 独立分集制作计划、补集队列、发布记录、`ProductionReport` |
| `/workbench/analytics` | 运营分析 / ANALYTICS | `ProductionReport` 与 `BackupReport`；趋势分析在 Phase 4 |
| `/workbench/exceptions` | 异常与决策 / EXCEPTIONS | 完整需要关注列表和现有编辑审稿界面 |
| `/workbench/assistant` | AI 助手 / ASSISTANT | 现有 `WorkbenchChat`，保留提案确认和操作审计 |
| `/workbench/me` | 我的 | 工作台账户和密码管理，固定在侧栏底部 |

所有子路由继承 `/workbench` 布局中的本机请求检查与编辑会话检查；未登录时显示现有登录页。页面 loader 只读取本页所需接口，不再每页并发请求 11 个 feed 接口。

## 旧链接重定向

旧入口统一在 `/workbench` 布局路由转换。`world`、`plan`、`task`、`candidate` 保留；新的详情参数 `item_key`、`run_key`、`source_table`、`source_id`、`episode`、`worker`、`loop` 也会保留。

| 旧链接 | 新路由 | 保留的详情参数 |
| --- | --- | --- |
| `/workbench?view=data`、`?view=data&section=data` 或 `?section=data` | `/workbench` | `world`、`plan`、`task`、`candidate` |
| `?view=data&section=knowledge` 或 `?section=knowledge` | `/workbench/knowledge` | `world`、`plan`、`task`、`candidate` |
| `?view=data&section=media&candidate=N` 或 `?section=media&candidate=N` | `/workbench/knowledge?candidate=N` | `candidate`、`world` |
| `?view=data&section=history` 或 `?section=history` | `/workbench/operations` | `run_key`、`source_table`、`source_id` 等详情参数 |
| `?view=editor&task=ID` | `/workbench/exceptions?task=ID` | `task`、`world` |
| `?view=chat&world=W&plan=P` | `/workbench/assistant?world=W&plan=P` | `world`、`plan`、`task`、`candidate` |
| `?view=me` | `/workbench/me` | 无 |
| 只带 `?section=knowledge`、`?section=media` 或 `?section=history` | 知识资产或运营拓扑 | 支持的详情参数 |

仓库内原有深链逐项迁移如下：编辑任务审稿到 `/workbench/exceptions?task=…`；分集计划和对话到 `/workbench/assistant?world=…&plan=…`；执行日志到 `/workbench/operations`；图片候选到 `/workbench/knowledge?candidate=…`。`apps/web/test/workbench-legacy.test.mjs` 覆盖以上映射及参数保留。

## Overview API

`GET /api/v1/admin/knowledge/ops/overview` 在现有管理员分组中，继续使用 Bearer 管理令牌。返回：

```json
{
  "as_of": "计算时间",
  "metrics": {
    "active_works": { "value": 0, "definition": 1, "updated_at": "来源更新时间" },
    "work_types": { "value": [{ "type": null, "count": 0 }], "definition": 1, "updated_at": "来源更新时间" }
  },
  "attention": []
}
```

每项 metric 都有 `value`、`definition`、`updated_at`。无快照时快照类 `value` 与 `updated_at` 为 `null`，界面显示「—」。`as_of` 是本次查询时间，不代替每项的来源更新时间。定义版本当前为 1。

| 指标 key | 值与口径 | 来源 / 更新时间 |
| --- | --- | --- |
| `active_works` | `works.active = true` 的作品数 | 最新全库 `ops_daily_snapshots`；`taken_at` |
| `published_worlds` | `published_worlds` 行数 | 同一全库快照；`taken_at` |
| `work_types` | 活跃作品按 `works.type` 原值分组；NULL 单列为「未登记」 | 同一全库快照；`taken_at` |
| `characters`、`portraits`、`chapters`、`chapter_images`、`storylines`、`scenes`、`locations` | 已发布作品世界的知识资产分项计数，分别列出，不相加 | `reader_knowledge_inventory` 的全库日快照；`taken_at` |
| `online_workers` | `last_ok_at` 在 `max(3 × expected_seconds, 120 秒)` 内的不同 `worker_id` 数；`consecutive_errors ≥ 3` 仍算在线，另在关注列表标为持续出错 | `ops_worker_heartbeats`；本次 `as_of` |
| `running_executions` | Agent 心跳 `executions` 中 media、research、editorial、configuration 的非终态数量之和 | `ops_worker_heartbeats` 的 Agent 行；本次 `as_of` |
| `running_tasks` | `ops_task_runs.status = 'running'` 的行数 | `ops_task_runs`；本次 `as_of` |
| `pending_work` | `ops_work_items.state = 'pending'` 的行数 | `ops_work_items`；本次 `as_of` |
| `blocked_work` | `ops_work_items.state = 'blocked'` 的行数 | `ops_work_items`；本次 `as_of` |
| `waiting_human` | `ops_work_items.state = 'waiting_human'` 的行数 | `ops_work_items`；本次 `as_of` |
| `completed_today` | `finished_at` 位于北京时间当天 `[00:00, 次日 00:00)` 且 `status = 'success'` 的执行数 | `ops_task_runs`；本次 `as_of` |
| `failed_today` | 同一北京时间区间内 `status = 'failed'` 的执行数 | `ops_task_runs`；本次 `as_of` |

快照指标来自最新全库行，不使用当前页面数据窗口，也不回填快照前历史。在线错误行按 Phase 0 的心跳规则计入在线进程，失败执行和待处理项则分别来自两个 Phase 0 视图。

### 需要关注

Overview 的 `attention` 最多 20 条，按严重程度与时间排序。`GET /api/v1/admin/knowledge/ops/attention` 使用同一来源与排序返回完整列表，供「异常与决策」页面使用。每行字段：`id`、`kind`、`severity`、`stage`、`work`、`world_slug`、`subject`、`reason_code`、`since`、`link.page`、`link.params`。没有作品或时间来源的行对应字段为 `null`。

| 来源 | 进入条件 | 严重程度 / 原因码 | 默认详情路由和参数 |
| --- | --- | --- | --- |
| 心跳 | 在线判定为离线或在线但连续错误不少于 3 次 | `critical` / `worker_offline`；`high` / `worker_consecutive_errors` | `/workbench/operations`；`worker`、`loop` |
| 待处理视图 | `blocked` 或 `waiting_human` | `high` / 原 `reason`；`info` / 原 `reason` | 依阶段进入 exceptions、knowledge 或 projects；带 `item_key`、`world`、`episode` 及 task / plan / candidate |
| 执行视图 | 最近 24 小时 `status = 'failed'` | `medium` / `execution_failed` | 依来源表进入对应页面；带 `run_key`、`source_table`、`source_id`、`world`、`episode` 和适用的 task / candidate |

等待审核的编辑任务链接进入异常页并预选该任务；图片候选用 `candidate` 选中；计划链接用 `plan` 定位；运行日志用运行键定位。页面保留 Phase 0 的任务、准入、发布保护语义。

## `status_code` 前置迁移

迁移 `060.sql` 在 `episode_policy_queue` 末尾添加稳定的 `status_code`，中文 `status` 继续作为展示值。Go、准入 SQL、`ops_work_items` 映射改用 code；未知 code 仍立即失败。迁移另增加只读 `ops_attention_items` 视图，供总览与完整关注列表共用 Phase 0 来源。

## 本阶段边界

运营拓扑仍复用现有节点推断，计数在 Phase 3 切换到拓扑接口；知识资产不新增完整度规则；运营分析不生成趋势；异常不写入持久异常台账；AI 助手不扩展工具。界面主题沿用 `board.css`。
