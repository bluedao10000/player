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
- 同源已有记录优先保留 UUID。跨源只有 provider + external_id 唯一定位且共有 provider 无矛盾时才自动匹配。没有可靠映射的记录获得独立 UUID。多个候选、互相冲突的 provider ID，或仅标题与年份一致的条目进入候选表。审核依据可以从当前记录的 raw、norm 和来源版本追溯。治理管理员可通过下述接口作出审核决定。
- AOD 优先提供展示标题、集数、状态和季节；Bangumi 提供原名、译名和日期。canonical 日期取上游时间戳的 UTC 日期，原始播出时刻仍保留。缺少剧情简介时 synopsis 为 NULL。标题、标签、来源、制作方等按所有有效贡献去重。
- 删除是源级撤回。一条记录从快照消失后标为 inactive，仍有其他来源的作品继续提供服务，仅撤去已消失的贡献。所有来源撤回后保留 UUID 并退出公开查询。相关作品 URL 暂无本地目标时保留 URL；后续出现目标后，关系视图可返回其 UUID。
- `knowledge_source_versions` 保留成功版本、SHA-256、metadata 和许可；`knowledge_source_records` 保存每个来源键最近一次的 raw 与 norm。治理迁移后的版本另有按 SHA-256 寻址的上游原文件。备份时同时保存 PostgreSQL 和归档卷。

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

## Phase 1 数据治理

**审计。** 迁移前的同步器已经有下载、Schema 校验、Normalize、staging、确定性身份匹配和事务发布。源记录只保存最近值，来源优先级写在合并 SQL 中，候选只读，缺少评分、字段覆盖和人工操作历史。H5 的作品检索页消费 Gin；原创页面的八张 WebP 由静态路径引用，数据库未登记图片与实体的关系、授权、版本或处理历史。后续媒体阶段再接入这些图片。

**版本和恢复。** 新增迁移 003。成功版本保存归档键、适配器类型和归档内文件名。同步器在提交前把已校验的原文件写入持久卷，原子替换同摘要文件；同一内容只占一份。PostgreSQL 保存版本引用和最近原始记录。事务失败可能留下无引用的文件，清理时需先核对版本引用和正在运行的任务。升级后首次探测当前版本会补取一次归档。升级前已经消失且未备份的发布物无法还原。历史重建按当前解析器重新归一化，并将该来源的当前投影切换到所选版本，下一次正常同步仍可推进至最新版本。

**人工决定。** 字段投影先取人工值，再按字段来源优先级、来源默认优先级取有效值，缺少年份时可从开始日期推导。高值优先；字段优先级表使用 norm 字段名。API 使用 works 字段名。撤销覆盖后重新计算当前值。变更、审核和同步共用事务锁，操作人、前后值、原因、时间和入口写入操作日志。原始数据接口只读。

| 治理表 | 用途 |
| --- | --- |
| knowledge_field_priorities、work_field_overrides | 来源优先级与独立人工值 |
| knowledge_identity_pins、knowledge_match_rejections | 保存已确认的归属及拒绝决定，供后续同步使用 |
| knowledge_quality_rules、knowledge_quality_issues、knowledge_quality_scores | 规则开关、按作品和规则聚合的异常、九项评分明细 |
| knowledge_curation_actions | 字段和身份操作审计 |

### 管理与命令行

Compose 重建 api 和 sync 后可使用下列命令。原文件位于命名卷 knowledge-archive。本机可用 KNOWLEDGE_ARCHIVE_DIR 指定绝对路径。数据库 CLI 使用数据库凭据；HTTP 管理接口要求配置单一管理员身份 KNOWLEDGE_ADMIN_OPERATOR 和至少 32 字节的 KNOWLEDGE_ADMIN_TOKEN，通过 Authorization Bearer 提交。给不同审核者分别分配身份和凭据后再开展多人审核。

```sh
docker compose exec api knowledge quality --source all
docker compose exec api knowledge quality --work-id "$WORK_UUID"
docker compose exec api knowledge review list
docker compose exec api knowledge review approve "$CANDIDATE_ID" "已对照两个来源的作品 ID 和发行记录"
docker compose exec api knowledge diff --source anime-offline-database --from "$OLD_VERSION" --to "$NEW_VERSION" --record "$RECORD_KEY"
docker compose exec api knowledge project --source anime-offline-database
docker compose exec api knowledge rebuild --source anime-offline-database --version "$VERSION"
```

project 从当前 source records 重算作品；rebuild 从已归档的上游文件重跑解析、匹配及投影，不指定版本时使用当前版本。diff 按原始记录键输出两版发生变化的顶层字段和前后值。质量扫描可按来源或单部作品执行，重复扫描将已消失的问题标为 resolved。检查和评分只写治理表。常驻 knowledge-sync 每轮检查质检是否到期，有活动作品时自动扫描，成功后六小时再执行，失败十五分钟后再试。专用数据库会话锁防止多个 Worker 重复扫描，重启会标记中断批次。单次 -once 导入沿用原有语义。GET /api/v1/knowledge/quality-runs 返回运行状态及报告，/workbench 通过只读 API 观测。

管理 API 前缀为 `/api/v1/admin/knowledge`。

| 请求 | 用途 |
| --- | --- |
| GET /versions?source=来源名 | 版本、SHA-256、归档可用性 |
| POST /quality?source=all 或 ?work_id=UUID | 执行扫描并返回范围统计 |
| GET /works/:id/issues、GET /works/:id/history | 评分分项、问题与操作历史 |
| GET /matches、GET /matches/:id | 待审核候选、两侧原始字段和差异 |
| POST /actions | 显式提交审核，成功返回 204 |

actions 的 JSON 必须提供 action 和 reason。override 接收 work_id、field、value；undo 接收 work_id、field，移除该字段的人工覆盖。approve 把单条记录归入候选作品；merge 把该作品所有来源记录归入目标；reject 排除该配对。三者使用 candidate_id。split 按 record_id 拆到新 UUID，也可用 work_id 指定该记录在合并历史中的旧 UUID。rematch 按 record_id 解除人工匹配约束并重建候选供复核。无贡献的 UUID 保留，退出活动作品查询。字段修正和身份操作完成后重新扫描相应作品，可查看最新分数。

### 数据质量报告

2026-11-18 在开发库的隔离副本上扫描 39,033 部活动作品，平均 66.38 分。每个数字是触发对应规则的作品数，同一作品可以计入多项。

| 规则 | 作品数 |
| --- | ---: |
| 同一来源记录中同语种别名重复 | 7,542 |
| 缺少封面来源线索 | 581 |
| 多部作品共用 provider ID | 425 |
| 同一作品的 provider ID 冲突 | 286 |
| 标题空白需规范化 | 5 |

本次其余规则为零，覆盖必填类型、空标题、无效来源 URL、语言标签、日期顺序、枚举值、未解析关联和异常集数。解析器此前拒收的记录不在本报告内。检查项可作为复核线索，多个发行条目共用同一个外部 ID 需要查证后再处理。

总分 100，标题、Provider 身份各 15 分，中文标题、原文标题、日期、类型、已批准图片、可信来源、关联完整性各 10 分。各项实际分值以 JSON 保存。当前已批准图片项统一为零；封面 URL 仅用于发现线索，不能证明图片可用或获准发布。

### AI 辅助判同与原创图片试稿

配置 MATCH_AI_BASE_URL、MATCH_AI_MODEL、MATCH_AI_API_KEY 后，管理员可向 `POST /api/v1/admin/knowledge/matches/{id}/assessments` 请求一次判断。数据发送至管理员选定的 OpenAI 兼容服务。请求包含双方标题、别名、来源 ID、发行时间、类型和集数。服务需要支持 chat/completions 和 JSON 输出。Jev 可改用原生 TypeSafe 接口：设置 MATCH_AI_PROVIDER=typesafe、TYPESAFE_API_KEY，TYPESAFE_MODEL 默认 jev-latest。服务端调用 POST /v1/systemone，将名称是否为别名、是否属于不同篇章作为两个 Noul 问题。程序保留完整问题、输入和返回概率，以及实际回答的模型名。初始 0.8 / 0.2 阈值尚待标注样本验证；判同还需 ID 或年份与类型佐证，冲突转人工。图片生成仍由 MEDIA_AI 配置的独立服务处理。

判断有 same、different、uncertain 三种。审核时先看作品的发行形态，再核对是否续季、总集篇或重制版。每条引用必须能定位到双方的同一字段，语义解释由审核者核实。GET 同路径可查历次建议、证据原文、模型、请求人及证据是否仍然有效。同证据、同模型、同提示词版本复用已存结果。最终接受或拒绝仍走人工 actions。上线评估建议用人工双标样本分别统计误合并、漏匹配与无法判断的比例，特别检查同名重制和不同语言发行版。

原创美术可先调用独立服务生成私人试稿。从 apps/api 运行下列命令，向进程提供 MEDIA_AI_BASE_URL、MEDIA_AI_MODEL、MEDIA_AI_API_KEY。生成服务需要支持 images/generations、指定尺寸和 b64_json PNG 响应。命令只在主动执行时请求一张图片。结果写入用户缓存目录，控制台输出本次目录。图库发现、参考图权利审批、正式资产入库和 CDN 发布仍按媒体阶段实施。

```sh
go run ./cmd/media generate-preview --brief /path/to/brief.json --size 1024x1536
```

原创角色试稿 brief 示例：

```json
{
  "original_subject": "白帆港夜班邮差阿澪，原创角色概念试稿",
  "visual_bible": "本次试稿设定：深色短发、青灰色眼睛、深蓝邮差制服、旧帆布邮包",
  "scene": "雨刚停，她在渡船舷梯前辨认一封信的邮戳",
  "composition": "竖幅半身，脸位于上半部，右手持信清晰，底部留出对白空间",
  "art_direction": "手绘动画背景，港灯暖光与海面冷光分离，制服轮廓在夜色中可辨"
}
```

**美术验收**分为文件检查与画面评审。试稿工具会解码整张 PNG，检查实际尺寸，重新编码，并记录提示词、模型和文件摘要。评审时把同一角色的候选与已确认的视觉设定并排，在原尺寸查看五官、手部、配饰和场景连续性，再按目标页面裁切预览，检查手机首屏的人脸、叙事动作与对白留白。难以认出角色、手部遮挡关键动作、关键主体被裁掉或疑似复刻第三方海报，均应退回并记录具体返工原因。技术检查通过只代表文件合格。正式角色图采用获准参考资产与版本化自动审核标准；不确定的视觉项提交 Jev，数据服务根据结果与预算决定重裁、重生成或结束。发布记录按具体用途留存。见[三层架构与图片优化](部署架构.md#三层架构中的图片优化)。

### 从已清洗档案到正式知识版本

本节之后的本地验收覆盖当前两个规则样板。目标流程在资料层之上增加运营选题、AI 联网取证、四类叙事编纂、图片处理与五维联合验收。普通作品可以长期保留已清洗档案；正式版按作品版本和覆盖范围独立发布。[知识库构建整体流程](作品世界样板/知识库清洗/流程说明.html) · [作品状态卡与验收口径](产品架构.md)

### 作品知识状态（只读）

工作台通过 `GET /api/v1/knowledge/world-statuses?limit=30` 查看公开世界的当前版本，按 slug 排序。单部作品（含基础档案）走 `GET /api/v1/knowledge/works/{uuid}/status`。非法 UUID 返回 400，未知或失效的作品返回 404。

PostgreSQL 视图 `work_knowledge_status` 复用当前版本指针和出场关系的外键约束。只有同一版本存在可追溯的角色、章节、场景及段落关系时，成熟度才为 `linked`。图位统计是引用库存。当前前两集的事件顺序、五维凭证和已发布范围由独立的范围验收接口提供。[完整构建流程](知识库构建流程.md)

### 作品世界的本地验收

首页「作品世界」和阅读路径从 Gin 读取当前发布的数据。原创清单为 [tidepost.json](../apps/api/internal/knowledge/worlddata/tidepost.json)。外部作品的中文拟题、导读和待核对关系在 [cowboy-bebop-tv.json](../apps/api/internal/knowledge/worlddata/cowboy-bebop-tv.json)。清单随数据服务镜像发布。

```sh
docker compose up -d --build postgres api sync
docker compose exec sync knowledge-sync -once -source tidepost-original
docker compose exec sync knowledge-sync -once -source bebop-tv-guide
npm run dev:web
```

第二条来源需要基础同步已建立 Bangumi 条目 253 对应的 1998 TV 版作品；空库应先完成上方基础数据源同步。常驻 worker 按来源周期自动执行。外部任务从 Bangumi 获取前两集、角色标识和原名，以官方分集简介及第 2 集的补充资料核实出场关系；缺失或冲突会记为失败并保留先前发布版本。中文导读、场景命名来自站内编辑清单。同步记录以一个作品世界为一条记录，版本元数据保存本次通过验证的清单。支持 HTTP 条件请求的来源会复用文档缓存。

打开 [H5 首页](http://127.0.0.1:4310/) 依次点击角色、出场章节、阅读这一幕、回到故事。在 [首页数据接口](http://127.0.0.1:4321/api/v1/knowledge/worlds?placement=home) 查看发布版本，在[数据工作台](http://127.0.0.1:4310/workbench) 查看同步记录。实际表使用来源版本和稳定 slug 作为子实体复合键，段落使用章内稳定键；Gin 通过一个已发布版本指针读取完整关系。定时、幂等、互斥、失败重试与运行历史复用 knowledge-sync 的现有机制。

### 第二阶段 · 《星际牛仔》1998 TV 第 1、2 集取证

在根目录 .env 配置 `RESEARCH_OPENAI_API_KEY`，可选 `RESEARCH_MODEL`（默认 gpt-4.1-mini）。运行 `docker compose up -d --build`；H5 沿用现有的本地 Web 服务。样板导入后，腾讯云数据服务对应的本地 `sync` 进程按当前作品知识版本检查选题。也可执行 `docker compose run --rm sync knowledge-sync -research` 立即处理到期选题。已有成功结果时跳过；失败可提前重试，每份作品版本、选题规则最多三次尝试。运行需要访问官方分集页、英文分集指南和 OpenAI 文本模型。

- 工作台打开 `/workbench` 的「分集取证过程」。可核对工作版本、取证规则、执行次数、抓取时间、来源段落及指纹、串联结果与失败原因。`GET /api/v1/knowledge/research-runs` 提供实际记录。
- H5 从 `/work/cowboy-bebop-tv` 进入章节。读者页展示连贯的剧情正文，旧场景的段落锚点保留。取证接口保留逐段来源、引文及位置。
- Go 数据服务拥有范围、运行、证据、校验与预览。Pi Durable 在独立容器通过受限工具实时读取两个指定站点，按事件问题组织证据、写稿并复核。每次调用有持久请求 ID，Agent 卷保存运行检查点，PostgreSQL 保存业务结果。新来源排版或事件定位对不上时记录失败。再次成功的规则版本有独立运行历史。
- 本期的十个事件问题及出处定位为前两集专门设计，材料正文与导读来自运行时。前两集的五维结果与场景定稿见工作台「范围验收」，全作不在本次范围内。

### 第三阶段 · 阿澪场景试稿

本地数据工作台的「阿澪 · 场景试稿」展示港口日景半身、雨夜侧脸、渡口持信三组。每组的三个候选都可以打开原图。右侧在 390 × 844 的嵌入视口渲染角色页首屏；「验图样张」保存送给视觉模型的合成裁切。展开任务记录可查看历次生成、退回原因、修订和复验结果。公开角色页仍使用原立绘。

Compose 的 sync 服务定时从 PostgreSQL 领取任务，分派给 Pi Durable 的图片任务。Agent 使用角色页的 mio-invitation.webp、已登记指纹和 OpenAI 图像编辑接口，生成 1024 × 1536 PNG。初次构图采用角色页的 65% / 40% 焦点。数据服务据验图结果决定通过、重新裁切、重绘或保留未通过的试稿。每个候选最多两版原图，每版最多重裁一次。视觉检查比对立绘、原图和预览，检查人物、剧情动作、人体结构、画面文字及裁切。初检报告缺陷时由第二个视觉模型独立复核。模型结论用于候选验收，正式发布另有流程。

配置 MEDIA_WORKER_ENABLED=true 和 MEDIA_AI_API_KEY、RESEARCH_OPENAI_API_KEY，运行 docker compose up -d --build。图像模型使用 MEDIA_AI_MODEL，视觉初检及复核分别使用 MEDIA_REVIEW_MODEL 和 MEDIA_SECOND_REVIEW_MODEL。三个服务无须共享模型密钥，模型调用位于 Agent。原图与派生图在 media-assets 卷中，Pi 检查点在 research-state 卷中，任务与审核历史在 PostgreSQL。重启后沿用同一候选、版本、步骤和尝试号；已结束的批次会跳过。验收后按需要备份三个卷。

《星际牛仔》的公开阅读页展示剧情、人物、世界观与分集。两集的中文润色稿绑定到各自的两份来源快照，来源变化后回到当前取证稿。出处保留在取证记录中。

**《星际牛仔》1998 TV 前两集场景试稿。** 工作台的 `/workbench#bebop-media` 独立展示「药瓶赴约」「宠物店开箱」「街头遛狗」各三个候选。该批次先由数据服务通过 Pi Durable 生成一张斯派克的非官方人物研究图。九个候选复用同一个持久参考图任务，生成与复核都读取同一文件并校验指纹。横版候选原图为 1536 × 1024，工作台右侧使用场景页封面组件，在 390 × 844 手机视口展示 358 × 225 的封面。点击标题旁的缩略图可以打开参考图，展开候选记录可以核对验图和修订历史。批次、版本及候选位置共同区分任务及文件，既有阿澪试稿和公开阅读页保留。
