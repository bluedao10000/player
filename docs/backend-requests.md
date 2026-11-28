git apply <<'DEEPWORKS_PATCH_EOF'
diff --git a/outputs/workbench-acceptance/backend-requests.md b/outputs/workbench-acceptance/backend-requests.md
new file mode 100644
index 0000000..b8fd7c4
--- /dev/null
+++ b/outputs/workbench-acceptance/backend-requests.md
@@ -0,0 +1,45 @@
+# 待后端补齐的接口与数据清单
+
+> 摘自 docs/api-contract.md §11（2026-11-25 联调后整理），可直接转发后端。
+
+
+前端已按"无 mock"原则完成 real 模式改造：凡后端无对应接口/数据的展示位一律隐藏或显示 N/A，不再用演示数据补齐。以下是让各展示位恢复完整功能所需的 backend 工作，按优先级排序：
+
+| # | 缺口 | 影响的展示位 | 建议接口/工作 | 前端现状 |
+|---|------|------------|--------------|---------|
+| 1 | 作品画像字段全 null | 作品知识表格：完整度/已验证实体/缺口 三列 | 填充 `GET /works` 行内 `workbench_profile`（completeness、verified_entity_count、gap_count），或提供 `GET /works/:id/profile` | 三列显示 N/A / — |
+| 2 | 知识库概览统计 | 作品知识页顶部四格概览 | `GET /stats/overview`：已发布知识版本数、已验证知识项总数、开放缺口数、质量问题数 | 显示 N/A（待统计端点） |
+| 3 | 执行记录聚合统计 | 执行记录页顶部统计条 | `GET /runs/stats?from&to`：completed/failed/cancelled 计数、成功率 | 前端在已拉取页内聚合（数据量大后会失真） |
+| 4 | 成本核算 | 执行记录"总成本"卡片、Usage 面板成本 | runs 落库时记录 `usage.cost_usd`（token × 单价）；可选 `GET /runs/stats` 返回 total_cost | 总成本卡片已移除；Usage 成本显示 — |
+| 5 | 简报趋势序列 | 今日简报 InsightCards（知识增长/AI 成本 7 天趋势） | `GET /trends?metric=knowledge_growth\|ai_cost&days=7` → `{ points: [{time, value}] }` | 趋势卡已移除 |
+| 6 | 简报推荐行动为空 | 今日简报"AI 推荐行动"区 | `GET /briefing` 生成 `actions[]`（含 intent、可选 decision_id、evidence_refs 供"查看依据"） | 区块整体隐藏 |
+| 7 | 决策队列为空 | 决策收件箱整页 | 决策产生管线（pipeline 触发 approval 类决策落库） | 空态文案 |
+| 8 | Agent run 不落库 | 执行记录页看不到 agent.chat 记录 | `POST /agent/runs` 执行落库为 run（kind=agent.chat），含事件时间线与 usage；另需提供 `POST /agent/runs/:id/cancel`（前端"停止"目前仅断开 SSE，后端推理仍在计费） | 仅展示本地会话内的取消记录 |
+| 9 | 聚合型 artifact 结果集 | AI 工作区 works-table / metric-comparison / coverage-matrix / cost-breakdown | 按 §8.1 提供对应结果集端点 | 降级占位组件 |
+
+说明：#4 与 #5 涉及成本口径，建议后端先确认 token 计费数据来源（模型网关还是自统计）再定接口形状；前端展示位已预留，接口就绪后接入成本约为各半个工作日。
+
+---
+
+# 第二批：简报数据需求（2026-11-25，产品决策：简报以作品知识库为中心）
+
+背景：前端已完成简报页重构——核心指标换为知识四指标（已验证知识项/全库平均完整度/待补缺口/质检问题），新增「今日执行概览」按任务类型分组（来源同步 20 次 = 16 完成 / 4 失败；AI 对话 29 次 = 3 完成 / 26 手动停止），风险按来源聚合去重并显示真实失败原因（runs.error_summary）。以下数据前端无法自行获得，需要后端提供：
+
+| # | 需求 | 建议接口/字段 | 用途 | 优先级 |
+|---|------|--------------|------|--------|
+| 10 | 按作品的今日知识增量 | `GET /stats/knowledge-delta?date=YYYY-MM-DD` → `{ rows: [{ work_id, title, new_verified_items, by_kind: { character?, chapter?, scene?, relation? }, source_run_ids: [] }] }` | 简报新增「今日知识增量」区块：哪部作品今天补了多少知识（如「Naruto +12 角色」）。**用户明确只要按作品这一维度** | 高 |
+| 11 | 已验证知识项的今日增量（delta） | 方案 A：`GET /briefing` metrics 每项加 `delta` 字段；方案 B：trends `knowledge_growth` 日快照落库（目前 7 个点全 null） | KPI「已验证知识项」显示 +N 今日增量；趋势卡恢复 | 高 |
+| 12 | 简报结构化执行摘要 | `GET /briefing` 增加 `execution: { total, by_type: [{ task_type, total, by_status: {} }] }`（当日，北京时） | 替代前端全量拉 1430 条 runs 客户端聚合的临时方案；也让 `summary_tokens` 的生成有结构化依据（目前 "51 次执行其中 22 完成 4 失败" 对不上账：25 次 cancelled 未交代，已被用户指出看不懂） | 中 |
+| 13 | 风险项携带失败原因与聚合 | `GET /briefing` risks 每项加 `error_summary`、`occurrences`（同源失败次数）；或服务端直接按来源聚合去重 | 前端目前已自行聚合 + 从 runs 反查 error_summary，跨日时仍依赖后端 | 中 |
+| 14 | 平均完整度的服务端口径 | `GET /stats/overview` 增加 `avg_completeness`（全部已发布作品 completeness 均值） | 前端目前拉 100 条作品客户端求均值，作品数增长后失真 | 低 |
+
+说明：#10 是「今日知识增量」区块的唯一数据源，接口就绪后前端接入约半个工作日；#12/#13 前端已有客户端兜底实现，属于性能与口径稳定性优化。
+
+> **状态更新（2026-11-26 00:40）**：#11–#14 后端已交付并接入——`briefing.execution`（当日执行按类型×状态分组，服务端口径）、risks 直出 `detail`/`occurrences`/`error_summary`、`stats/overview.avg_completeness`、metrics `delta`。前端已切换为服务端口径（撤销客户端拉全量 runs 聚合与 works 求均值的临时方案）。#10 `GET /stats/knowledge-delta` 端点已上线，返回结构正确但 rows 为空（尚无增量落库），前端区块已就绪、有数据即自动出现。注意：`/docs/api-contract.md` 当前 404，契约文档入口疑似被移除，请后端确认。
+
+> **#15（2026-11-26，✅ 已交付并接入）**：`GET /works` 返回范围从「工作台作品」（契约 §4 示例 total:24）扩大为全量来源目录 39,120 条，其中仅 25 部有已验证知识（verified_entity_count>0，与 stats/overview.published_versions 一致），其余 39,095 条携带空 workbench_profile。作品知识页因此被稀释：分页器显示 39,120，空画像行显示整页 N/A。后端采用方案 A：`processed=true` 只返回 verified_entity_count>0 的作品（total 同步为过滤后数量 25），契约 §4 已补录。前端已接入：作品知识页默认开启「仅看已处理作品」（可切换查看全量目录 39,120），@ 引用源同样只列已处理作品。
+
+> **新增 #16–#18（2026-11-26，作品 Drawer 迭代配套）**：
+> - **#16 范围型缺口**：`GET /works/:id/gaps` 对「分集覆盖 · N」类批量缺口直出聚合形态（如 `{ kind: "episode_range", range: [1, 220], count: 215 }`），避免 215 条同构记录；前端已做客户端折叠过渡，接口就绪后切换。
+> - **#17 成熟度口径**：Naruto 完整度 14% 但 `maturity_level=established`（成熟），两口径矛盾感强。请核对 maturity 评定是否应纳入知识检查项通过率，或在契约 §4 写明两口径定义与关系（前端已加说明 tooltip 过渡）。
+> - **#18 缺口人话描述**：gap 增加 `detail_human` 字段（面向运营的描述，如「该集暂无已验证的分集内容，待来源同步补齐」），替代当前的系统态文案（「尚无当前版本的公开分集内容」）；前端已做模板映射过渡。
DEEPWORKS_PATCH_EOF