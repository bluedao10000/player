# PostgreSQL Anime Knowledge Base

Gin、TanStack Start 和 Pi Durable 读取同一套 canonical UUID。只有独立的 Go worker 检查和下载上游。作品查询在上游不可用时仍然读取上次成功发布的 PostgreSQL 数据。

## 启动

需要 Docker Compose。首次启动时，从仓库根目录复制 `.env.example` 为 `.env`，填写随机生成的 `POSTGRES_PASSWORD`（例如 `openssl rand -hex 24`）。已有部署沿用已初始化卷的密码。数据库连接另支持外部 PostgreSQL 的 `DATABASE_URL`。

```sh
docker compose up -d --build
docker compose logs -f sync
```

PostgreSQL 17 使用命名卷 `knowledge-data`。默认映射本机 5434 端口；容器内 Gin 使用 4311，宿主机 API 使用 4321。服务和 worker 运行于独立容器并配置自动重启，Docker 引擎需运行。第一次同步期间查询接口可以返回空集合。查看 `/api/v1/knowledge/sync-runs` 判断进度和失败原因。完整导入需要为 PostgreSQL 数据、索引、WAL 和事务临时数据预留磁盘空间。

```sh
curl 'http://127.0.0.1:4321/api/v1/knowledge/works?q=Cowboy%20Bebop&limit=5'
curl 'http://127.0.0.1:4321/api/v1/knowledge/sync-runs?limit=4'
docker compose exec sync knowledge-sync -once -source bangumi-data
```

现有开发环境用 `npm ci && npm run dev` 启动 H5、本地 Gin 和 Pi。H5 搜索入口是 [/knowledge](http://127.0.0.1:4310/knowledge)。H5 的 server function 和 Pi 的工具通过 `@anime/shared/knowledge` 调用 `KNOWLEDGE_API_URL`，默认指向 Compose Gin。本地 Gin（4311）也连接同一 PostgreSQL。Pi 原有的本地运行时文件仅保存对话执行检查点。

## 上游契约

| 适配器 | 版本检查与数据校验 | 解析与归一化 |
| --- | --- | --- |
| anime-offline-database | GitHub release 的 tag、asset ID 和 SHA-256 构成版本。先检查小型 metadata，再下载指定资产，核对字节数和摘要。滚动标签在下载期间换版会导致本批失败重试。 | 首行 metadata 单独保存，后续逐行使用仓库内固定的 JSONL entry schema 验证，包括必填字段、枚举与 URI 格式。解析来源 URL / ID、类型、别名、季节、制作方、标签和相关作品 URL。 |
| bangumi-data | 向 npm 官方 registry 查询已发布 package 的 latest 版本，确认 main，下载同源 tarball 并验证 SHA-512 integrity。版本不写死。 | 仅读取归档内的 package/dist/data.json，有解包大小限制。按 siteMeta 解析 info / onair / resource，原始条目完整保留。info ID 用于匹配，onair 的实际 URL、时刻、周期、地区与原始播出信息保存到 broadcasts。 |

AOD JSON Schema 取自 cedya77 仓库提交 `d96517ebea2975b916c36e2c4c24c6633ec281db`。Bangumi 依据实际 npm 发布物的 `data.d.ts` 和 `dist/data.json`；首次接入验证的包版本为 `0.3.237`。上游新增可选字段保留在 raw，必填字段或已支持枚举不兼容时，修订适配器后可重新验收。

[AOD 项目与许可](https://github.com/cedya77/anime-offline-database) · [JSONL schema](https://github.com/cedya77/anime-offline-database/blob/d96517ebea2975b916c36e2c4c24c6633ec281db/schemas/anime-offline-database.jsonl.schema.json) · [Bangumi 项目与许可](https://github.com/bangumi-data/bangumi-data) · [官方 npm 发布包](https://www.npmjs.com/package/bangumi-data)

## 批次、发布与所有权

1. Worker 每分钟检查本地到期状态，默认每个源每六小时探测一次远端。每个源有独立 PostgreSQL session advisory lock，跨进程生效。人工和定时任务都使用同一实现。
2. 每次实际探测先创建 run。版本不变写入 success / version_unchanged；人工触发遇到正在运行的同源任务记作 already_running。下载到临时文件，验证长度、大小上限及摘要，再解析。
3. 每 400 行使用 PostgreSQL COPY 写入 `staging_source_records`。run UUID 同时是 batch ID，查询运行中的 run 可看到已入 staging 的记录数。重复 record_key、无效 schema、空批次都会失败。
4. 验证 staging 数量，并与上次有效记录数比较。默认低于 80% 停止发布，此阈值按源配置。全局事务锁串行化各源的最终匹配与发布，避免两个源同时抢建 canonical ID。读者在事务提交前继续获得上一版。
5. 在同一事务中提交 `knowledge_source_versions`、原始与归一化记录、作品与关联表、候选项、来源指针、run 统计，并清理 staging。发布失败整体回滚。下载、校验或解析失败也不会修改作品。
6. 进程中止会释放 session 锁。下一个 worker 回收遗留的 running/pending 批次，将其记为失败并清理 staging。失败后最多等待 15 分钟再探测；更短的来源间隔照常生效。GitHub 对匿名请求限流时可在部署环境提供 `KNOWLEDGE_GITHUB_TOKEN` 并重建 sync 容器配置。

`knowledge_sources` 是来源 URL、检查端点、资产名称、开关及频率的配置入口。迁移按文件版本、在锁和事务内应用，不覆盖用户配置。新增同类型数据源可配置一行；新增传输格式需要增加检查、解析适配器和对应测试。

## 作品身份与合并

- 每个来源记录有独立身份。AOD 优先选取可信 provider 键；Bangumi 包含共用作品 ID 的配音及发行条目，因此记录键组合 provider、语言、类型、首播时刻。缺少资讯站 ID 时使用标题作为回退。重复键使整批失败，避免静默覆盖。
- 同源已有记录优先保留 UUID。跨源只有 provider + external_id 唯一定位且共有 provider 无矛盾时才自动匹配。没有可靠映射的记录获得独立 UUID。多个候选、互相冲突的 provider ID，或仅标题与年份一致的条目进入候选表。审核依据可以从当前记录的 raw、norm 和来源版本追溯。本次没有引入自动模糊合并或审核写接口。
- AOD 优先提供展示标题、集数、状态和季节；Bangumi 提供原名、译名和日期。canonical 日期取上游时间戳的 UTC 日期，原始播出时刻仍保留。缺少剧情简介时 synopsis 为 NULL。标题、标签、来源、制作方等按所有有效贡献去重。
- 删除是源级撤回。一条记录从快照消失后标为 inactive，仍有其他来源的作品继续提供服务，仅撤去已消失的贡献。所有来源撤回后保留 UUID 并退出公开查询。相关作品 URL 暂无本地目标时保留 URL；后续出现目标后，关系视图可返回其 UUID。
- `knowledge_source_versions` 保留成功版本、SHA-256、metadata 和许可；`knowledge_source_records` 保存每个来源键最近一次的 raw 与 norm。版本日志不是历史全量快照备份。按时用 `pg_dump` 备份可恢复 PostgreSQL 全库。

## 接口

| API | 返回内容 |
| --- | --- |
| GET /api/v1/knowledge/works?q=名称&limit=20&after=UUID | 原名与别名子串搜索，UUID 游标分页，limit 为 1–100 |
| GET /api/v1/knowledge/works/{UUID} | 作品字段、名称、来源 ID、标签、制作方、播出、关联作品与来源版本/许可 |
| GET /api/v1/knowledge/sources | 启用状态、检查间隔、当前版本和最后检查/成功时间 |
| GET /api/v1/knowledge/sync-runs?source=bangumi-data&limit=20 | 最新批次的版本、HTTP 状态、摘要、读入与增改删计数、跳过或失败原因 |
| GET /api/v1/knowledge/match-candidates?limit=20 | 待核对的记录、canonical UUID 与匹配原因；完整集合在同名表 |

Pi 注册只读工具 `search_anime_works` 和 `read_anime_work`。前者返回作品 UUID，后者返回有数量限制的设定资料与来源，供后续启用的模型使用。作品元数据 H5 页面使用 SSR 和 noindex，继续沿用产品将作品索引与编辑发布分开的设计。

## 运行维护与验证

```sh
# 稳定版本只检查，适配器更新后可明确指定 -refresh 重建当期投影。
docker compose exec sync knowledge-sync -once -source anime-offline-database
docker compose exec sync knowledge-sync -once -source bangumi-data -refresh

# 查询配置与审核队列
docker compose exec postgres psql -U anime -d anime_knowledge
# 示例 SQL
# UPDATE knowledge_sources SET sync_interval=interval '12 hours',updated_at=now() WHERE name='bangumi-data';
# SELECT reason,count(*) FROM knowledge_match_candidates GROUP BY reason;

# 一份独立于 Docker 卷的备份
docker compose exec -T postgres pg_dump -U anime -Fc anime_knowledge > anime_knowledge.dump

npm run check
```

PostgreSQL 集成测试用 `KNOWLEDGE_TEST_DATABASE_URL` 指定可创建 schema 的测试数据库，运行 `go -C apps/api test ./internal/knowledge -count=1 -v`。每次测试建立独立 schema 并回收。覆盖双源明确匹配、同名候选、provider 冲突、重复探测、损坏下载、已部分 staging 后解析失败、事务提交前拒绝、重试、批量减少保护、源撤回、遗留批次回收及同源锁竞争。默认测试路径始终运行 AOD schema/parser 测试。
