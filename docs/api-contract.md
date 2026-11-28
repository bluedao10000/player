# Folio AI 数据工作台 · 前后端接口契约

版本：v1.0（2026-11-25） · 前端基线：folio repo `adacfef` 起 · 状态：**后端已部署本地，待前端联调**

> 本文档是后端开发的唯一接口依据。前端数据层（`src/features/workbench/data/api.ts`）已实现 mock/real 双模式，real 模式按本文档逐端点对接；前端类型定义见 `src/features/workbench/types.ts`，与本文档字段一一对应。

---

## 1. 总体约定

| 项 | 约定 |
| --- | --- |
| Base URL | `/api/v1`（开发与生产均经代理访问，前端不感知后端真实地址） |
| 数据格式 | JSON；`Content-Type: application/json; charset=utf-8` |
| 流式接口 | `POST /agent/runs`，`text/event-stream`（SSE），见 §8 |
| 时间 | ISO 8601 字符串，**一律带时区**（业务时间为北京时间 `+08:00`），如 `2026-11-25T09:30:00+08:00` |
| 金额 | 数值 + `currency` 字段（ISO 4217，如 `USD`/`CNY`），前端不假设币种 |
| 命名 | 响应字段 snake_case（与本文档及前端 types 一致） |
| 鉴权 | 本地联调请求由同源代理转发 `Authorization: Bearer <token>`。令牌保存在代理服务端 |
| 幂等与乐观锁 | 审批类写操作携带 `expected_revision`；不匹配返回 `409 revision_conflict`，**不得自动覆盖**，见 §6 |

### 1.1 错误模型

所有非 2xx 响应统一错误信封：

```json
{
  "error": {
    "code": "revision_conflict",
    "message": "提案已被更新（rev 8），请重新比较后再提交",
    "details": { "current_revision": 8 }
  }
}
```

| HTTP | code（示例） | 语义 |
| --- | --- | --- |
| 400 | `invalid_argument` | 参数非法（details 指明字段） |
| 401 | `unauthenticated` | 未登录/令牌失效 |
| 403 | `forbidden` | 无权限 |
| 404 | `not_found` | 资源不存在 |
| 409 | `revision_conflict` | 乐观锁冲突（审批必须重新比较） |
| 410 | `decision_expired` | 决策已过期，不可再处理 |
| 429 | `rate_limited` | 限流（带 `Retry-After` 头） |
| 500 | `internal` | 服务端错误 |
| 503 | `upstream_unavailable` | 依赖不可用（前端展示"数据服务暂不可用 + 重试"） |

### 1.2 数据纪律（后端必须遵守）

- **不允许伪造指标**：无数据时数值字段返回 `null`（前端展示 `N/A`），不得返回 `0`；
- 百分比/覆盖率在**分母不可靠时不计算**，对应字段返回 `null`；
- KPI 必须带口径说明（`note` 字段，强制）；
- 列表接口未命中返回空数组/`{ "rows": [], "total": 0 }`，不报错。

---

## 2. 今日简报

### `GET /briefing`

生成（或读取缓存的）当日简报。简报是**主体内容**，不是 Dashboard 附属。

**Query**：`date?`（`YYYY-MM-DD`，默认今天，北京时间）

**响应 200** `Briefing`：

```json
{
  "generated_at": "2026-11-25T09:12:00+08:00",
  "data_until": "2026-11-25T06:00:00+08:00",
  "summary_tokens": ["过去 24 小时系统整体健康。", "完成 4 项知识生产任务，", "……"],
  "metrics": [
    {
      "key": "active_works",
      "label": "活跃作品",
      "value": 24,
      "unit": null,
      "delta": "+2",
      "deltaGood": true,
      "note": "近 30 天有知识变更的目录作品",
      "drillTo": { "page": "knowledge", "params": {} }
    },
    {
      "key": "verified_items",
      "label": "已验证知识项",
      "value": 2268,
      "delta": "+66",
      "deltaGood": true,
      "note": "通过证据校验的实体+关系总数",
      "drillTo": { "page": "knowledge", "params": {} }
    }
  ],
  "risks": [
    {
      "id": "risk-1",
      "level": "high",
      "title": "Bangumi 增量同步连续 429",
      "detail": "06:00 窗口连续 7 次失败，今晚全量窗口存在风险",
      "decision_id": "d-1042"
    }
  ],
  "actions": [
    {
      "id": "act-1",
      "title": "合并「芙莉莲」重复角色实体",
      "rationale": "置信度 0.91，影响 214 条引用，回滚可用",
      "decision_id": "d-1042",
      "intent": "解释合并芙莉莲重复角色的影响"
    }
  ]
}
```

字段规则：
- `summary_tokens: string[]` —— executive summary 的词级序列（前端按 55ms/词流式播放；拼接后即为完整摘要）；
- `metrics[].value: number | null` —— `null` → N/A；`note` **必填**（口径说明）；`drillTo.page ∈ today|inbox|knowledge|activity|chat`；
- `risks[].level ∈ high|mid|low`；`decision_id` 存在时前端提供"去处理"跳转；
- `actions[].decision_id` 存在 → 前端展示"转收件箱"；否则展示"让 AI 执行"并以 `intent` 发起对话。

---

## 3. 决策收件箱

### `GET /decisions`

**Query**：`status?`（`pending|approved|rejected|snoozed|expired`，缺省全部）、`risk?`（`L1|L2|L3`）

**响应 200** `DecisionItem[]`（按 `created_at` 倒序；字段见 §3.2）

### `GET /decisions/:id`

**响应 200** `DecisionItem`（含 `assessment`/`diff` 等详情字段；列表接口可省略这两个大字段）

**404**：`not_found`

### `POST /decisions/:id/resolve`

审批动作。**前端不直接修改业务状态**，只提交意图 + 乐观锁版本，由后端执行并返回回执。

**请求**：

```json
{ "action": "approve", "note": "可选备注（驳回时必填理由）", "expected_revision": 7 }
```

- `action ∈ approve|reject|snooze`；`reject` 时 `note` 必填（400 `invalid_argument`）；
- `expected_revision` 与当前不一致 → **409 `revision_conflict`**（details 带 `current_revision`），前端刷新最新提案要求人工重新比较；
- 已过期 → **410 `decision_expired`**。

**响应 200**：更新后的 `DecisionItem`（`approve` 成功时 `run_id` 指向执行回执）。

### `DecisionItem`

```json
{
  "id": "d-1042",
  "title": "合并「芙莉莲」重复角色实体（3 → 1）",
  "description": "第二季采集引入了 3 个疑似重复的角色实体",
  "status": "pending",
  "risk_level": "L3",
  "action_type": "entity.merge",
  "proposal": { "merge_into": "char-frieren", "merge_from": ["char-f1", "char-f2"] },
  "expected_revision": 7,
  "confidence": 0.91,
  "affected_works": 2,
  "affected_entities": 3,
  "affected_records": 214,
  "evidence_count": 6,
  "rollback_available": true,
  "rollback_summary": "按快照回滚合并前的实体与引用",
  "created_at": "2026-11-24T21:36:00+08:00",
  "expires_at": "2026-11-27T21:36:00+08:00",
  "assessment": "三个实体在名称、罗马音、声优、出场作品四个维度完全一致……",
  "diff": [
    { "field": "entity.char-frieren.name_cn", "kind": "mod", "before": "芙莉莲", "after": "芙莉莲（规范）" },
    { "field": "entity.char-f1", "kind": "del", "before": "芙莉莲(重复)", "after": null },
    { "field": "relation.r-8821.target", "kind": "mod", "before": "char-f1", "after": "char-frieren" }
  ],
  "work_id": "w-frieren-s2",
  "run_id": null,
  "resolution_note": null
}
```

- `status ∈ pending|approved|rejected|snoozed|expired`；`risk_level ∈ L1|L2|L3`；
- `diff[].kind ∈ add|del|mod`（`before`/`after` 为字符串化值，可空）；
- **L3 决策不提供批量接口**（规格书红线）。

---

## 4. 作品知识

### `GET /works`

**Query**：`search?`（标题/原标题模糊）、`type?`、`status?`（`published|draft|archived`）、`sort?`（`gap|updated|coverage`，默认 `updated`）、`page?`（0 起，默认 0）、`page_size?`（默认 10）、`processed?`（`true` 时只返回 `verified_entity_count>0` 的作品；默认 `false` 返回全量目录）

**响应 200**：

```json
{ "rows": [ /* WorkKnowledgeSummary[] */ ], "total": 24 }
```

### `GET /works/:id` → `WorkKnowledgeSummary`

```json
{
  "work_id": "w-frieren-s2",
  "title": "葬送的芙莉莲 第二季",
  "original_title": "葬送のフリーレン",
  "cover_url": null,
  "work_type": "TV 动画",
  "release_year": 2026,
  "publication_status": "draft",
  "verified_entity_count": 21,
  "character_count": 34,
  "chapter_count": 12,
  "event_count": 58,
  "relation_count": 47,
  "coverage": 0.31,
  "coverage_status": "partial",
  "gap_count": 34,
  "quality_issue_count": 7,
  "maturity_level": "growing",
  "updated_at": "2026-11-25T08:41:00+08:00"
}
```

- `coverage: 0-1 | null`（分母不可靠时 `null`，前端 N/A）；`maturity_level ∈ seed|growing|established|reference`。
- `maturity_level` 描述作品在发布/关联/覆盖上的阶段，不等同于 `completeness`：`seed` 未发布且未关联；`growing` 已发布但未关联读者清单；`established` 已关联读者清单但覆盖或缺口未清零；`reference` 已关联且 `coverage=1`、`gap_count=0`。`completeness` 是知识检查项通过率，两者独立，可能出现 `established` 但完整度较低。
- `processed=true` 只返回已有已验证知识的作品；`total` 同步为过滤后的数量。全量目录仍需浏览时省略该参数。

### `GET /works/:id/gaps` → `WorkGap[]`

```json
[
  {
    "id": "gap-1",
    "kind": "missing_entity",
    "label": "角色「尤贝尔」缺少规范实体",
    "detail": "在第 3/5/7 章被引用，但未建立角色档案",
    "detail_human": "该知识项尚未补齐，需从来源同步或制作流程补充。",
    "severity": "high"
  },
  {
    "id": "episode_range:1-220",
    "kind": "episode_range",
    "label": "分集覆盖 · 1–220",
    "detail": "共 215 集缺少当前版本的公开分集内容。",
    "detail_human": "第 1–220 集范围内有 215 集暂无已验证的分集内容，待来源同步补齐。",
    "severity": "high",
    "range": [1, 220],
    "count": 215
  }
]
```

- `kind ∈ missing_entity|weak_evidence|incomplete_relation|timeline_anomaly|missing_image|source_conflict|episode_range`；`severity ∈ high|mid|low`。
- `detail_human` 是面向运营的中文描述；`detail` 保留系统态原文。
- 批量「分集覆盖 · N」缺口（≥2 集）按作品聚合为一条 `episode_range`，带 `range:[首集,末集]` 与 `count`；单集缺口仍返回普通 `missing_entity`。

### `GET /works/:id/recommendations` → `WorkRecommendation[]`

```json
[
  {
    "id": "rec-1",
    "action": "补全角色「尤贝尔」档案",
    "rationale": "被 3 个章节引用，证据 ev-101",
    "estimated_cost": 0.42,
    "currency": "USD"
  }
]
```

---

## 5. 执行记录

### `GET /runs`

**Query**：`status?`（`queued|running|completed|failed|cancelled|waiting_approval`）

**响应 200** `AgentRunSummary[]`（按 `started_at` 倒序）

### `GET /runs/:id` → `AgentRunSummary`

```json
{
  "run_id": "r-2642",
  "task_name": "葬送的芙莉莲 第二季 · 知识抽取",
  "task_type": "knowledge.extract",
  "status": "completed",
  "started_at": "2026-11-25T06:03:00+08:00",
  "finished_at": "2026-11-25T06:11:24+08:00",
  "duration_ms": 504000,
  "input_tokens": 128400,
  "output_tokens": 9600,
  "cost": 0.58,
  "currency": "USD",
  "tool_count": 6,
  "related_work_id": "w-frieren-s2",
  "related_decision_id": null,
  "result_summary": "新增 21 个验证实体、47 条关系",
  "error_summary": null
}
```

### `GET /runs/:id/events` → `AgentRunEvent[]`（按 `sequence` 升序）

```json
[
  {
    "event_id": "ev-1",
    "run_id": "r-2642",
    "sequence": 1,
    "type": "tool_call",
    "timestamp": "2026-11-25T06:03:02+08:00",
    "tool_name": "catalog.fetch_chapters",
    "status": "ok",
    "summary": "拉取章节列表（12 章）",
    "duration_ms": 1800,
    "artifact_refs": []
  }
]
```

- `type ∈ thinking|tool_call|tool_result|decision|error|note`；`status ∈ ok|failed|retrying`；
- 历史任务允许只有摘要没有事件（返回空数组，前端展示"仅有摘要"）。

---

## 6. 证据

### `GET /evidence` → `EvidenceItem[]`

```json
[
  {
    "id": "ev-1",
    "kind": "query",
    "title": "验证实体增长口径",
    "detail": "SELECT count(*) FROM entities WHERE verified_at > now() - interval '24h'",
    "ref": "d-1039",
    "timestamp": "2026-11-25T09:10:00+08:00"
  }
]
```

- `kind ∈ query|api|source|file|receipt`；`ref` 为关联对象 id（run_id / decision_id / work_id）。

---

## 7. 知识计划（Plan Artifact 数据源）

### `GET /plans/:id` → `KnowledgePlan`

```json
{
  "plan_id": "p-2026w48-frieren",
  "title": "葬送的芙莉莲 第二季 · 知识补全计划",
  "status": "draft",
  "objective": "两周内把覆盖率从 31% 提升到 70%",
  "target_work_ids": ["w-frieren-s2"],
  "estimated_cost": 7.6,
  "currency": "USD",
  "milestones": [
    { "title": "角色档案补全", "due_at": "2026-11-30T00:00:00+08:00", "status": "pending" }
  ],
  "tasks": [
    {
      "task_id": "t-1",
      "title": "补全 5 名核心角色档案",
      "priority": "high",
      "estimated_cost": 2.1,
      "status": "pending",
      "dependencies": []
    }
  ],
  "expected_revision": 1
}
```

- `status ∈ draft|pending_approval|approved|running|completed|cancelled`；`priority ∈ high|mid|low`；
- 计划批准走收件箱（生成 `DecisionItem`），**不在本接口直接改状态**。

---

## 8. Agent 编排（SSE）—— chatbox first 的核心接口

### `POST /agent/runs`

发起一次对话/任务编排。请求：

```json
{
  "intent": "为葬送的芙莉莲制定知识补全计划",
  "model": "folio-agent",
  "conversation_id": null,
  "context": {
    "page": "knowledge",
    "scope": "work",
    "tags": [{ "id": "page", "label": "页面", "value": "knowledge", "removable": false }],
    "mentions": [{ "kind": "work", "id": "w-frieren-s2" }]
  }
}
```

**响应 200**：`Content-Type: text/event-stream`，每帧 `data: <json>\n\n`，事件序列：

```
run_started → thinking_step* → tool_call* → token* → artifact* → follow_ups? → done
                                                                        ↘ error
```

| event.type | 载荷 | 前端消费 |
| --- | --- | --- |
| `run_started` | `{ run_id, conversation_id }` | 绑定会话与执行记录 |
| `thinking_step` | `{ id, label, detail? }` | ThinkingState 逐步呈现 |
| `tool_call` | `{ id, tool, label, status: running|done|error, duration_ms? }` | ToolCall 列表（同一 id 可多次推送以更新 status） |
| `token` | `{ t: "……" }` | 正文流式（词级，前端 55ms/词节奏播放） |
| `artifact` | `{ artifact: { type, ref } }` | 结构化卡片，见 §8.1 |
| `follow_ups` | `{ items: string[] }` | 追问建议（点击即以该文本发起新 intent） |
| `done` | `{ run_id, usage?: { input_tokens, output_tokens, cost, currency } }` | 编排终态 |
| `error` | `{ code, message, retryable? }` | 错误态（前端展示重试） |

流结束可用 `data: [DONE]` 或直接关闭连接。**断线恢复**：前端可用 `GET /agent/runs/:run_id` 查询状态、用 `GET /runs/:run_id/events` 回放补齐（规格书 §7.6）。

### `GET /agent/runs/:run_id` → `AgentRunSummary`（复用 §5 结构）

### 8.1 Artifact 类型（`ChatArtifact`）

`artifact.type ∈ works-table|plan|metric-comparison|approval|coverage-matrix|cost-breakdown`，`ref` 为取数引用：

| type | ref 指向 | 前端补充取数 |
| --- | --- | --- |
| `works-table` | 作品查询快照 | `GET /artifacts/works-table/:ref` |
| `plan` | plan_id | `GET /plans/:id` |
| `metric-comparison` | 简报指标快照 | `GET /artifacts/metric-comparison/:ref` |
| `approval` | decision_id | `GET /decisions/:id` |
| `coverage-matrix` | 作品完整度快照 | `GET /artifacts/coverage-matrix/:ref` |
| `cost-breakdown` | 执行费用快照 | `GET /artifacts/cost-breakdown/:ref` |

后端只需下发 `{ type, ref }`，前端按上表取数渲染；取数失败时前端展示降级占位而非报错。

### 8.2 停止生成

前端通过 `AbortController` 断开 SSE（用户点"停止"）。后端检测到连接断开应**尽快停止推理并落盘已完成部分**，run 状态标记 `cancelled`，已产出内容可通过 events 回放。

---

## 9. 端点总览

| 方法 | 路径 | 用途 | 前端调用点 |
| --- | --- | --- | --- |
| GET | `/api/v1/briefing` | 今日简报 | `fetchBriefing` |
| GET | `/api/v1/decisions` | 决策队列 | `fetchDecisions` |
| GET | `/api/v1/decisions/:id` | 决策详情 | `fetchDecision` |
| POST | `/api/v1/decisions/:id/resolve` | 审批（乐观锁） | `resolveDecision` |
| GET | `/api/v1/works` | 作品列表（搜索/筛选/分页） | `fetchWorks` |
| GET | `/api/v1/works/:id` | 作品详情 | `fetchWork` |
| GET | `/api/v1/works/:id/gaps` | 知识缺口 | `fetchWorkGaps` |
| GET | `/api/v1/works/:id/recommendations` | AI 建议 | `fetchWorkRecs` |
| GET | `/api/v1/runs` | 执行记录列表 | `fetchRuns` |
| GET | `/api/v1/runs/:id` | 执行记录详情 | `fetchRun` |
| GET | `/api/v1/runs/:id/events` | 执行事件流 | `fetchRunEvents` |
| GET | `/api/v1/evidence` | 证据列表 | `fetchEvidence` |
| GET | `/api/v1/plans/:id` | 知识计划 | Plan artifact 取数 |
| POST | `/api/v1/agent/runs` | Agent 编排（SSE） | `streamAgentRun` |
| GET | `/api/v1/agent/runs/:id` | Agent 运行状态 | 断线恢复 |

---

## 10. 联调与部署

### 10.1 前端双模式

| 模式 | 开关 | 行为 |
| --- | --- | --- |
| mock（默认） | `VITE_API_MODE=mock` 或不设 | 数据走 fixtures，前端可完全独立开发/演示/跑测试 |
| real | `VITE_API_MODE=real` | 数据走 `VITE_API_BASE`（默认 `/api/v1`） |

前端的端点封装位于 `src/features/workbench/data/api.ts`，SSE 客户端位于 `src/features/workbench/agent/stream.ts`。

### 10.2 本地联调（开发）

```bash
# 根 Compose 的 Gin API 默认监听 127.0.0.1:4321
VITE_API_MODE=real VITE_PROXY_TARGET=http://127.0.0.1:4321 pnpm dev
```

Vite dev server 把 `/api` 代理到 `VITE_PROXY_TARGET`（见 `vite.config.ts`）。代理进程读取后端本地 `KNOWLEDGE_ADMIN_TOKEN`，向上游请求添加 Bearer 头，并透传 SSE。浏览器仍使用同源 `/api/v1`。本地契约为 [HTTP Markdown](http://127.0.0.1:4321/docs/api-contract.md)。

### 10.3 生产部署（NGINX）

```
pnpm build            # 产物 dist/
# dist/ → /var/www/folio；NGINX 配置见 deploy/nginx.workbench.conf
```

要点：
- 静态托管 `dist/`，`location / { try_files $uri $uri/ /index.html; }`（前端为 hash 路由，无 history API 依赖）；
- `location /api/ { proxy_pass http://127.0.0.1:8080; }` 反代后端；
- **SSE 必须**：`proxy_buffering off; proxy_read_timeout 300s;`（配置中已含）；
- 前端与 API 同源，生产同样无 CORS。

### 10.4 联调节奏建议

1. 后端先实现 §2–§6 只读接口 + 错误模型，前端 `VITE_API_MODE=real` 逐页验证；
2. 实现 `resolveDecision`（含 409/410），验证审批状态机；
3. 实现 `POST /agent/runs` SSE（先支持 `token`/`done` 最小序列，再补 thinking/tool/artifact）；
4. 接入 WorkbenchContext 的 real 编排（前端任务，替换 `scripts.ts` 调度）。

---

## 11. 工作台联调补充

以下端点沿用 `/api/v1`、Bearer 鉴权和统一错误信封。时间窗使用左闭右开，缺少可靠测量时数值返回 `null`。

| 方法 | 路径 | 响应 |
| --- | --- | --- |
| GET | `/works/:id/profile` | `{ work_id, completeness, verified_entity_count, gap_count }`。作品列表与详情内的 `workbench_profile` 包含后三个字段。 |
| GET | `/stats/overview` | `{ published_versions, verified_items, open_gaps, quality_issues, avg_completeness, as_of }`。 |
| GET | `/stats/knowledge-delta?date=YYYY-MM-DD` | `{ rows: [{ work_id, title, new_verified_items, by_kind, source_run_ids }] }`，按作品列出当日首次进入已验证版本的知识项。 |
| GET | `/runs/stats?from&to` | `{ from, to, completed, failed, cancelled, success_rate, total_cost, currency }`。 |
| GET | `/trends?metric=knowledge_growth\|ai_cost&days=7` | `{ points: [{ time, value }] }`。点按北京时间日界线升序，`days` 取 1–90，缺省 7。 |
| POST | `/agent/runs/:id/cancel` | 200，返回该执行的 `AgentRunSummary`。重复取消可安全重试。执行已进入其他终态时返回当前状态。 |
| GET | `/artifacts/:type/:ref` | 按下面的结构读取 Agent 输出的聚合结果。 |

### 11.1 统计口径

- `completeness` 是当前发布版可测检查项的完成数除以适用项总数，范围为 0–1。草稿或缺少分母时返回 `null`。它与按可读集数计算的 `coverage` 分开。
- `verified_entity_count` 统计当前发布版有来源依据的角色、章节与场景。同作品同类对象按稳定标识去重。`verified_items` 在此基础上加上有依据的章节与场景出场关系。概览按作品统计当前有效发布版，`quality_issues` 只计活跃作品的未关闭质检问题。
- `runs/stats` 按结束时间归集。时间参数接受带时区的 ISO 8601 时间或北京时间日期，缺省为截至现在的最近 7 天。成功率为 `completed / (completed + failed)`，取消单列。缺少分母时为 `null`。
- `usage.cost_usd`、`total_cost` 和 AI 成本趋势使用去重的供应商回执及调用时已生效的 USD 成本费率。运行详情可随后到的回执更新。执行统计的总成本汇总能按执行归集的模型主体，趋势按供应商调用开始日期统计全部可计价模型主体。区间内相关回执或 USD 费率缺失时，金额为 `null`。
- `knowledge_growth` 是连续两个北京时间日快照之间可读章节库存的净增量，需要两天都有快照。负数表示库存净减少。`ai_cost` 的数值单位为 USD。
- `knowledge-delta` 按 `(work_id, kind, stable_key)` 去重，统计指定北京时间日内首次进入已验证版本的角色、章节、场景与有据出场关系。`source_run_ids` 使用 `/runs` 的执行 ID；没有增量的作品不占行。
- `avg_completeness` 先按当前发布作品计算 `completeness`，再对作品取算术平均；没有可测作品时返回 `null`。

### 11.2 简报与取消

`GET /briefing` 的推荐行动带 `intent` 与 `evidence_refs: string[]`。存在 `decision_id` 的行动跳转收件箱，有据缺口行动可把 `intent` 发送给 Agent。`evidence_refs` 使用 `GET /evidence` 中的记录 ID。来源同步发现有待核对的变动时创建决策，Agent 生成待审批计划时也会创建决策。

简报另返回 `execution: { total, by_type: [{ task_type, total, by_status }] }`，按北京时间开始日期汇总当日执行。`verified_items` 指标带 `delta` 与 `deltaGood`：今日为当前值减当日开始快照，历史日期为次日快照减当日快照；缺基线时两者均为 `null`。`risks[]` 带 `error_summary` 与 `occurrences`，失败风险按任务类型、任务名称和失败原因聚合，决策风险的 `occurrences` 为 1。

点击停止时可先请求 `POST /agent/runs/:id/cancel`，再断开 SSE。服务端保存取消标记并停止上游推理；仍连接的 SSE 收到 `done` 且 `status: "cancelled"`。已收到的事件可通过运行事件端点回放。

### 11.3 聚合结果集

使用 SSE 给出的 `type` 和 `ref` 请求 `GET /artifacts/:type/:ref`。新生成的结果保存在 PostgreSQL，后续打开同一个卡片读到对应快照。计划和审批仍分别通过 `/plans/:id` 和 `/decisions/:id` 读取。

| type | 响应 |
| --- | --- |
| `works-table` | `{ rows: WorkKnowledgeSummary[], total }` |
| `coverage-matrix` | `{ rows, total }`。每行是作品摘要，并附 `dimensions: [{ key, completed_required_items, applicable_required_items, completeness }]`。 |
| `metric-comparison` | `{ metrics: Briefing["metrics"], as_of }` |
| `cost-breakdown` | `{ run: AgentRunSummary, items: [{ call_id, started_at, model, input_tokens, output_tokens, cost_usd, currency }] }` |
