# PostgreSQL 作品知识库与治理

目录回答「这是哪一部、哪个发行版本」；作品世界回答「这一版中谁在哪一集出场、这幕回到哪段正文」。两者用 canonical 作品 UUID 关联。一次目录清洗可独立完成，生成可读知识需要运营选题和分集发布。[分集构建流程](知识库构建流程.md)

## 上游到 canonical

| 来源 | 更新探测 | 导入契约 |
| --- | --- | --- |
| anime-offline-database | GitHub Release metadata、资产标识与摘要 | 先校验下载，再按 JSONL 读取首行 metadata 与后续作品，按固定 schema 校验，提取 provider URL/ID、别名、季节、标签与制作方 |
| bangumi-data | npm 官方 registry 的包版本和 tarball integrity | 检查 main、同源下载与 SHA-512，提取 package/dist/data.json，区分 info ID 和播出来源 |
| 原创与作品样板 | 仓库内原创资料或带补充来源的作品清单 | 原创内容与外部资料分别记录来源和作品版本；维持稳定工作标识与路由 |

来源的检查地址、下载地址、开关与间隔在 `knowledge_sources`。适配器变更须核对 schema 与 fixture。上游项目及固定 schema 可从以下入口追查。[AOD](https://github.com/cedya77/anime-offline-database) · [Bangumi](https://github.com/bangumi-data/bangumi-data) · [适配器](../apps/api/internal/knowledge/upstream.go) · [解析器](../apps/api/internal/knowledge/parse.go) · [原创入库](../apps/api/internal/knowledge/world_import.go)

## 状态与事务

1. `knowledge_sync_runs` 记录一次探测的版本、状态、HTTP 信息、摘要、计数或跳过原因。同源 advisory lock 串行化同步。
2. 完整下载并校验后，解析结果写入按批隔离的 `staging_source_records`。空批、重复键、校验失败或保留比例异常阻止归并。读者继续读已提交版本。
3. 最终事务更新 `knowledge_source_versions`、`knowledge_source_records`、`works` 及标题、来源、关系、标签、制作、播出等投影。原始资产归档后可按摘要复核。
4. 可靠 provider + ID 用于判同；冲突及弱匹配进入 `knowledge_match_candidates`。上游撤回按来源生效，有其他有效来源的作品仍可查询。UUID、原始记录、治理覆盖各有独立生命周期。

[同步与失败恢复](../apps/api/internal/knowledge/sync.go) · [匹配与归并 SQL](../apps/api/internal/knowledge/merge.sql) · [投影](../apps/api/internal/knowledge/projection.sql) · [迁移](../apps/api/internal/knowledge/sql)

## 管理、回放与读取

- 查询作品走 `GET /api/v1/knowledge/works?q=名称` 和 `/works/{UUID}`；`/sources`、`/sync-runs`、`/quality-runs` 是同步与质检观察入口。定时质量扫描与分集的五维验收使用不同的记录与指标。[目录 API](../apps/api/internal/knowledge/api.go) · [质量调度](../apps/api/internal/knowledge/quality_schedule.go)
- 管理前缀为 `/api/v1/admin/knowledge`。候选的 assessments 接口可经 OpenAI 兼容服务或原生 TypeSafe Jev 返回判同依据。正式合并、拆分、覆盖与撤销经治理 action 写入审计记录。评估应结合对应候选的当前输入版本读取。[治理操作](../apps/api/internal/knowledge/curation.go) · [Jev 适配器](../apps/api/internal/knowledge/jev.go)
- 归档比对、重建与重投影由 `knowledge diff`、`knowledge rebuild`、`knowledge project` 提供。后两者会写库，恢复演练用隔离库和对应版本的归档。所需参数见 [CLI](../apps/api/cmd/knowledge/main.go) 与[运维手册](运维手册.md)。
