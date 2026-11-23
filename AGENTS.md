# Folio 时光 仓库入口

本仓库把二次元作品资料整理成能阅读和回访的作品世界。读者走 H5 → Gin → PostgreSQL，数据工作台沿同一路径观察运行记录。Go 数据服务发起取证、出图和验收，TypeScript Pi Durable 执行模型与工具步骤。先按任务选一篇文档，再沿文档给出的入口读代码。

| 你要做什么 | 先读 | 开始改动的位置 |
| --- | --- | --- |
| 判断功能归属与成熟度 | [产品架构](docs/产品架构.md)、[功能地图](docs/功能地图.md) | [H5 路由](apps/web/src/routes)、[共享契约](packages/shared/knowledge.ts) |
| 改 H5 阅读、角色出场、场景回跳 | [技术架构](docs/技术架构.md) | [世界页面](apps/web/src/features/worlds/pages.tsx)、[Gin 读模型](apps/api/internal/handler/knowledge/world_api.go) |
| 改 iOS 原生阅读或签名归档 | [iOS 工程与验收](apps/ios/README.md) | [SwiftUI 页面](apps/ios/Folio)、[公开阅读接口](apps/ios/server/reader.server.ts)、[XcodeGen](apps/ios/project.yml) |
| 改界面文字、按钮、提示或状态词 | [界面文案规范](docs/界面文案规范.md) | [H5 读者端](apps/web/src/features/reader)、[原创角色与故事](apps/web/src/features/post)、[SwiftUI 页面](apps/ios/Folio)、[工作台状态表](apps/web/src/features/workbench/shared/format.ts) |
| 接数据源、改匹配或数据质量 | [知识库与治理](docs/knowledge-base.md) | [来源适配](apps/api/internal/service/knowledge/upstream.go)、[归并事务](apps/api/internal/repository/queries/knowledge/queries.queries) |
| 登记作品分集制作计划 | [分集配置](docs/知识库构建流程.md)、[构建流程](docs/知识库构建流程.md) | [选题清单](apps/sync/episode-plan.example.json) |
| 改取证、自动验图或发布 | [知识库构建流程](docs/知识库构建流程.md) | [数据服务](apps/sync/cmd/sync/main.go)、[Agent 入口](apps/agent/src/server.ts) |
| 改一句话编辑、手机审稿或任务发布 | [构建流程](docs/知识库构建流程.md)、[运维手册](docs/运维手册.md) | [编辑调度](apps/api/internal/service/knowledge/editor_worker.go)、[批准事务](apps/api/internal/service/knowledge/editor.go)、[回归测试](apps/api/internal/handler/knowledge/editor_test.go) |
| 改模型接入、查 token、扣费或对账 | [模型用量与计费](docs/模型用量与计费.md) | [调用回执](apps/api/internal/service/modelusage)、[会员与供应商对账](apps/api/internal/service/billing)、[账务集成测试](apps/api/internal/handler/knowledge/billing_test.go) |
| 启动、定位故障、备份或规划双云 | [运维手册](docs/运维手册.md)、[网络与部署](docs/部署架构.md) | 本地根 Compose；部署用 [Compose](deploy/docker-compose.yml) 与[配置模板](deploy/.env-example) |
| 准备提交或更新文档 | [开发规范](docs/开发规范.md)、[实现核验](docs/架构实现核验.md) | [检查命令](package.json)、[文档目录](docs/README.md) |

## 工作约定

- 先定位任务入口、负责写状态的服务和一个可观察结果。修改响应时一起检查 Gin 查询、共享类型、TanStack loader 和页面。修改选题时先核对作品版本与集号。
- PostgreSQL 持有业务事实和发布指针。来源快照、候选稿、定稿分别存储。Pi 的 SQLite 用于恢复执行。保持已发布作品 UUID、slug、段落锚点以及入选图片的内容地址稳定。
- 延续当前工程组织。Web 与 Agent 使用 npm workspaces，Go 的 API 与同步器分别在 `apps/api`、`apps/sync` 构建，共用数据库迁移与业务包。新版本通过 [GORM 迁移注册表](database/migrations/migrate.go) 追加，业务 SQL 按领域放在 [apps/api/internal/repository/queries](apps/api/internal/repository/queries)，Go 通过命名的嵌入资源绑定查询参数，路由树由 TanStack 生成。模型密钥从服务端环境读取。
- 从最小反馈开始。可先跑目标包或测试，再运行根目录 `npm run check`；涉及迁移、合并和发布时加跑隔离 PostgreSQL 测试。H5 改动沿真实阅读路径同时检查 SSR 和手机端。具体命令、验证样例见上表。
- 更新能力状态时给出可复查的源码、测试或运行查询。历史评审、上游资料和接口回包用于理解上下文。当前行为以代码、迁移和可复现检查为准。
