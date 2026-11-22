# 运营中心 Phase 2：知识资产中心设计

设计稿，2026-11-22，已按产品评审结论定稿，现进入实现。依据[运营中心重构审计](运营中心重构审计.md)第 1.2、4、5、6 节和[Phase 0 设计](运营中心-Phase0-设计.md)第 5 节，以及[Phase 1 设计](运营中心-Phase1.md)。

## 1. 设计边界

- 不计算跨维度总分，不配置权重；每个维度只展示 `completed_required_items / applicable_required_items` 和状态。维度间不相加。
- `knowledge_completeness_rules` 由迁移维护，不提供编辑界面；质量扫描调度负责计算并写入 `knowledge_completeness_results`。
- 人物关系、组织 / 设定类型、时间线事件本阶段不建表。相应数据块显示「无数据」，不显示 `0%`，也不进入分母。
- 「待验证」必须有可追溯依据：存在源记录、取证或验收材料，且该规则明确要求的对应范围验收尚未通过。没有这类依据时按「缺失」或「无数据」处理，不以「待验证」兜底。
- 保留 Phase 0 当前业务表、视图语义和日快照主键；本稿不改变调度准入、发布保护或公开阅读结构。

完整度的五种计算状态为：`completed`（已完成）、`partial`（部分完成）、`missing`（缺失）、`not_applicable`（不适用）、`pending_verification`（待验证）。「无数据」是结构可用性，不是第六种完整度状态：结果另带 `data_state='no_data'`，该结果不带分子、分母，不产生百分比。一个维度中可计算的子项仍照常展示；例如人物介绍可计算时，人物关系另显示「无数据」。

## 2. 现状盘点

### 2.1 查询范围和限制

我只读查询了本机 Compose 开发库 `anime_knowledge`，查询连接强制设置 `default_transaction_read_only=on`，没有执行迁移或写入。库内 `knowledge_migrations` 当前最高为 `057.sql`，因此这份数据尚未包含已提交的 Phase 0/1 迁移对象（`059`、`060`、`ops_daily_snapshots` 等）。下列实际资产数取自该库已有的 `published_worlds`、`reader_knowledge_inventory` 及关联业务表；趋势历史和 Phase 0 快照在这个开发库中不可核验。计数表示查询时的本机开发数据，不代表线上生产库。

### 2.2 两个作品的实际数据

| 作品 | 当前版本 | 基础档案与来源 | 人物 | 剧情 | 场景 / 世界 | 图片 |
| --- | --- | --- | --- | --- | --- | --- |
| `cowboy-bebop-tv` | `version_id=9`；`world_releases` 1 条；外部作品 | TV，作品档案 26 集；`work_sources` 12 个外部 ID；2 条活跃来源记录（anime-offline-database、bangumi-data）；12 条 `world_evidence` | 8 人物；独立人物介绍 6 条；8 人物有立绘 | `reader_knowledge_inventory` 计故事线 9、章节 10、可读集 1–10；扩展分集 8 条，当前发布范围 9 个；分集制作计划 7 条 | 10 个可读场景 / 地点计数；底层 2 条场景有场景正文，另外 8 个分集扩展场景有名称、地点和章节关联，但没有独立场景正文字段 | 主视觉 1、人物立绘 8、章节配图 10 |
| `tidepost` | `version_id=8`；`world_releases` 1 条；原创作品 | 短篇集，`works.episodes` 为 NULL；无外部 `work_sources`，有 1 条 `tidepost-original` 活跃来源记录和 1 条 `world_evidence` | 3 人物；独立人物介绍 0 条；3 人物有立绘 | 故事线 1、章节 4；无可读分集；制作计划 0 条 | 2 个场景，2 个地点 | 主视觉 1、人物立绘 3、章节配图 4 |

`cowboy-bebop-tv` 的 8 人物包括 6 个当前世界版本人物和 2 个扩展分集人物。后两者在扩展内容中有出场文字，但没有独立的角色介绍记录；草案不把某一集的出场段落当成可复用的人物介绍。8 个分集扩展都已有正文及验收通过的发布范围。`tidepost` 的 4 个章节都有标题、导读和非空正文块。

开发库全局概况：活跃作品 39,036 个；类型分布为 MOVIE 6,686、ONA 6,461、OVA 4,807、SHORT_STORY_COLLECTION 1、SPECIAL 11,026、TV 9,940、UNKNOWN 115；当前没有 `works.type IS NULL` 的活跃作品。发布世界 25 个、`world_releases` 共 25 条。由此可见，目录作品远多于已形成发布知识版本的作品。

### 2.3 七个维度的表、字段和缺口

| 维度 | 现有数据依据 | 当前缺项 / 计数说明 |
| --- | --- | --- |
| 基础档案 | `works.canonical_title`、`type`、`episodes`、`synopsis`、`original_title`、状态和日期；`work_worlds.name`、`edition`、`intro`、`origin`、`version_id`；`world_releases`；`work_sources`、`knowledge_source_records`、`world_evidence` | 没有已统一的完整度规则。`works.episodes` 不能直接当作所有作品的章节数。 |
| 人物知识 | `work_characters.slug/name/role/evidence`；简介现存于 `edition_reader_intros.people` JSON。扩展人物从 `episode_extensions.people` 与 `published_episode_scopes.reader_copy` 派生；`published_characters`、`reader_knowledge_inventory` 提供当前人物和立绘计数 | `work_characters` 本身没有简介列；扩展人物的分集出场文字不是稳定的人物简介。人物关系没有专用表；`work_relations` 是作品 URL 关系，不是人物关系。 |
| 剧情知识 | `work_storylines.title/intro/position`、`work_storyline_chapters`；`work_chapters.name/intro/episode/position`、`chapter_blocks.text`、`edition_reader_chapters.passages`；扩展分集的 `episode_extensions`、已发布范围与 reader copy；作品的 `works.episodes` | `reader_knowledge_inventory.storylines` 还把已发布扩展分集计入故事线数；列表和完整度需沿用这个既有公开读数，并在规则中区分基础故事线与扩展分集，避免误把分集当成长期主线。 |
| 场景知识 | `work_scenes.name/text/place/chapter/block/evidence/position`；扩展场景在 `published_extensions.scene` 中有名称、地点、章节关联 | 扩展场景没有独立 `text` 字段。是否必须有场景正文，会把 Cowboy Bebop 的场景完整度从 10/10 改为 2/10，需评审确认。 |
| 世界观 | `work_settings(v,name)`；基础场景的 `work_scenes.place` 外键；扩展地点在 `episode_extensions.place`；库存视图将这些地点计入 `settings` | `work_settings` 只有名称，没有类型或描述；目前只能如实按地点名计数，不能区分组织或其它设定类型。 |
| 时间线 | 作品集号、章节顺序和场景位置存在于 `work_chapters.episode/position`、`work_scenes.position` | 没有事件实体、事件到章节的独立关联或事件先后校验记录。集号与显示顺序不能冒充剧情事件时间线。 |
| 图片资产 | `published_worlds.card.hero_asset_key`；`published_characters.card.portrait`；`published_chapters.card.image`；`edition_reader_art`；扩展发布卡片 | 这些都是当前发布版本可见的资产引用；私有候选和被撤下的图不计为已完成的发布图片。 |

## 3. 规则草案

### 3.1 规则与状态计算

规则表存机器可读的 `rule_code`、维度、项目代码与标签、对象范围、适用条件、评估器代码、是否要求范围验收、规则版本和显示顺序。表中不存任意可执行 SQL；`evaluator_code` 对应服务内经过测试的固定查询。规则变更由新迁移追加或更新规则版本，界面没有编辑权。每个 `item_code` 各自保存分子、分母、状态和明细；一个维度需要统计不同实体种类时按分项展示，不将分项分子、分母相加成一个比例。维度状态根据分项状态推导：所有分项均为无数据时为「无数据」；所有有数据分项均不适用时为「不适用」；有任一适用分项待验证时为「待验证」；所有适用分项已完成时为「已完成」；没有完成项且没有待验证项时为「缺失」；其余为「部分完成」。无数据和不适用分项不阻止其他分项完成，也不参加该维度状态的完成判断。

质量扫描对当前发布版本逐条评估，保存可复查的项目键、状态、证据来源和缺项原因。分子只计 `completed`；分母计所有适用项目，包括缺失和待验证项目；`not_applicable` 与 `no_data` 均不进分母。建议维度状态优先级：无可用结构为「无数据」；无适用项目为「不适用」；有待验证项目时为「待验证」；全部完成为「已完成」；完成数为 0 且无待验证为「缺失」；其它有缺项的情况为「部分完成」。缺失数和待验证数仍分别返回，避免状态优先级遮住明细。

只有当一个必需项目有明确的数据依据，并且对应 `scope_heads.latest` / `scope_acceptances` 尚未通过或尚未发布时，才记 `pending_verification`。例如分集已有 `episode_source_captures` 或 `research_runs` 证据，且存在相同 topic 的最新验收但 `passed=false`、没有通过的 `published` 指针。没有源记录、取证记录或可匹配的验收范围时，按缺失处理；不因“可能以后会补验收”而标为待验证。

### 3.2 各维度必需项

以下规则已按产品评审通过。每个计数单位在所属分项内按一个必需项计数；不同实体种类分开呈现，不合并分子 / 分母。`适用` 列为分母规则，明确不适用的项不进分母。

| 维度 / 项目 | 适用条件 | 完成判定（SQL 思路） | 缺失 / 待验证判定 |
| --- | --- | --- | --- |
| 基础档案：目录标题 | 所有 `works.active` | `nullif(btrim(works.canonical_title),'') IS NOT NULL` | 空为缺失；有待复核来源冲突时是否改为待验证，需按质量规则明确绑定，不能仅凭标题存在冲突的猜测。 |
| 基础档案：类型 | 所有 `works.active` | `works.type IS NOT NULL AND works.type <> 'UNKNOWN'` | NULL 与 `UNKNOWN` 均不算已登记类型，按缺失处理；类型分布仍按原值展示 `UNKNOWN`，NULL 单列「未登记」。 |
| 基础档案：版本展示信息 | 有 `work_worlds` 的作品 | `work_worlds.name`、`edition` 非空，当前版本指针存在，且对应 `world_releases(work_id,version_id)` 存在 | 有作品目录但尚无发布知识版本时不造 0 个知识项；行标记 `catalog_only/no_data`，不生成该版本的维度结果。 |
| 基础档案：简介 | 有发布知识版本的作品 | `works.synopsis`、`work_worlds.intro`、`edition_reader_intros.premise` 按明确优先级取首个非空值 | 全空为缺失；不将模型质量评分作为完成条件。 |
| 基础档案：来源可追溯 | 有发布知识版本的作品 | 外部作品需有有效 `work_sources` 或活跃 `knowledge_source_records`，并能关联该发布版本的 `world_evidence`；原创作品不要求外部 provider 链接，但需能追到原创来源记录 / `world_evidence` | 原创作品没有外部来源本身不算缺失。存在来源记录且明确等待验收才是待验证。 |
| 基础档案：总集数 | `TV`、`ONA`、`OVA`、`SPECIAL` 等分集类型 | `works.episodes > 0` | `MOVIE` 与 `SHORT_STORY_COLLECTION` 的分集项为不适用；缺类型不能推断为电影或无分集。 |
| 人物知识：人物介绍 | 每个当前发布人物 | `published_characters.slug` 对应的独立简介非空：`edition_reader_intros.people ->> slug`；扩展人物需未来有独立人物简介记录后才能计完成 | `work_characters` 没有简介字段；`extension_people.intro` 是分集出场段落，草案不拿它替代人物简介。每个可识别人物无独立简介为缺失。 |
| 人物知识：人物关系 | 需要人物关系数据的作品 | 暂无可执行判定 | 没有关系表，显示无数据，不建分母。 |
| 剧情知识：故事线 | 有当前发布版本 | 每条基础故事线需有标题、简介和章节链接；已发布扩展分集对应的故事单元沿用公开库存定义。比例只统计故事线 | 缺字段为缺失；有对应源 / 取证材料而最新范围未通过时待验证。 |
| 剧情知识：章节 | 有当前发布版本 | 基础章节要求 `work_chapters.name/intro` 非空且存在非空 `chapter_blocks.text` 或 `edition_reader_chapters.passages`；扩展章节要求有可读正文及非空简介。比例只统计章节 | 无正文为缺失；扩展正文已有来源但验收未通过时待验证。 |
| 剧情知识：分集覆盖 | 仅已知分集总数的连续剧类型；电影和短篇集不适用 | 每个 `1..works.episodes` 集号在当前公开可读集号中出现，并对应通过范围验收。比例只统计分集 | 有 `research_topics`、采集 / 取证证据和未通过的验收为待验证；没有该集数据为缺失。 |
| 场景知识：场景记录 | 每个当前发布场景 | 必须有非空名称、章节关联、地点和独立场景文本；基础场景读 `work_scenes.name/chapter/place/text`，扩展场景读发布扩展场景的独立文本字段。不得用章节正文或分集正文替代场景文本 | 扩展场景目前没有独立文本字段，按缺失计数并在缺项中列出；Cowboy Bebop 为 2/10。 |
| 世界观：地点 | 每个当前发布的地点 | 基础场景地点来自非空 `work_settings.name` 且被场景引用；扩展地点来自非空 `episode_extensions.place` 并关联公开扩展场景 | 不存在地点或名称为空为缺失；原创作品不要求 `work_sources` 外部链接。 |
| 世界观：组织 / 设定类型 | 需要这些分类的作品 | 暂无 `kind` 或组织实体字段可判定 | 显示无数据，不进分母；本阶段不在 `work_settings` 上临时猜类型。 |
| 时间线：事件、章节关联、顺序校验 | 需要剧情事件时间线的作品 | 暂无事件表、关系表或校验字段可判定 | 整个维度显示无数据，不以 `episode/position` 伪造事件。 |
| 图片资产：主视觉 | 有当前发布版本的作品 | `published_worlds.card->>'hero_asset_key'` 非空 | 为空为缺失。 |
| 图片资产：人物立绘 | 每个当前发布人物 | 对应 `published_characters.card->>'portrait'` 非空 | 为空为缺失；私有候选不算完成。 |
| 图片资产：章节配图 | 每个当前公开章节 | 对应 `published_chapters.card->>'image'` 或扩展章节公开图片非空 | 为空为缺失；被撤回或只有候选记录不算完成。 |

### 3.3 两个作品的试算

按评审结论，各项按实体种类分别展示分子 / 分母，不合成剧情或图片比例。人物关系、组织 / 设定类型和时间线等没有结构的项目显示「无数据」，不显示 0%，也不进分母。作品类型 `UNKNOWN` 不算类型登记完成。

| 维度 | Cowboy Bebop（`cowboy-bebop-tv`） | 潮汐邮局（`tidepost`） | 草案状态 |
| --- | --- | --- | --- |
| 基础档案 | 核心字段 7 / 7；总集数 1 / 1 | 核心字段 7 / 7；总集数不适用 | 已完成 / 已完成 |
| 人物知识 | 人物介绍 6/8；人物关系无数据 | 人物介绍 0/3；人物关系无数据 | 部分完成 / 缺失 |
| 剧情知识 | 故事线 9/9；章节 10/10；分集覆盖 10/26 | 故事线 1/1；章节 4/4；分集不适用 | 部分完成 / 已完成 |
| 场景知识 | 2/10：基础场景 2 条有文本；8 条扩展场景没有独立文本字段 | 2/2：名称、章节、地点和场景文本均可核对 | 部分完成 / 已完成 |
| 世界观 | 地点 10 / 10；组织和设定类型无数据 | 地点 2 / 2；组织和设定类型无数据 | 已完成（仅对有数据的地点项）/ 同左 |
| 时间线 | 无数据 | 无数据 | 无数据 / 无数据 |
| 图片资产 | 主视觉 1/1；人物立绘 8/8；章节配图 10/10 | 主视觉 1/1；人物立绘 3/3；章节配图 4/4 | 已完成 / 已完成 |

两例没有符合本稿定义的「待验证」项：Cowboy Bebop 当前可读的 9 个发布范围都已通过；潮汐邮局没有与其当前章节对应的 `research_topics/scope_heads`。有源证据但尚未通过范围验收的条目，将在实际数据出现时按 3.1 规则呈现。

## 4. 全库统计口径

| PRD 总览项 | 现有数据能否计算 | 建议口径 / 需要补充 |
| --- | --- | --- |
| 作品总数、类型分布 | 现在能算 | `works.active=true` 的数量；按 `works.type` 原值分组，NULL 单列「未登记」。 |
| 版本数 | 现在能算 | 每作品按 `world_releases` 记录数统计不可变发布版本；不把上游 `knowledge_source_versions` 快照或当前 `work_worlds.revision` 当作作品版本数。两个样例各 1 个。 |
| 人物总数 | 现在能算 | 沿用 `reader_knowledge_inventory.characters`，包括当前公开扩展人物；样例 8 / 3。 |
| 人物介绍完整数 | 现在部分可算 | 对应公开人物逐个检查独立简介；简介来源明确到 `edition_reader_intros.people` 等字段。样例 6/8、0/3。扩展出场文字不自动当简介。 |
| 人物关系数 | 等新结构 | 没有人物关系实体；不从共场或作品 URL 关系推导。 |
| 故事线、章节、分集、场景 | 现在能算 | 故事线与章节沿用 `reader_knowledge_inventory`；总集数用 `works.episodes`，覆盖集数用当前公开且通过范围验收的 `readable_episodes`；场景沿用库存计数。 |
| 地点 | 现在能算 | 只把可由 `work_settings.name`、场景地点和扩展 `place` 支持的地点计数。 |
| 组织、设定类型 | 等新结构 | `work_settings` 没有类型字段，也没有组织表；不分类猜测。 |
| 时间线事件、关联章节、顺序校验 | 等新结构 | 当前没有事件及其关系 / 校验记录。 |
| 主视觉、人物立绘、章节配图 | 现在能算 | 只计当前公开版本可见的资产引用；不计私有候选、撤下项或仅在候选表中的图。 |
| 知识完整度 | Phase 2 补上 | 依规则表计算当前发布版本的各维度分子、分母、状态和项目明细；无完整度总分。 |
| 趋势分析 | 部分能算；完整度趋势 Phase 2 补上 | 知识资产的既有日趋势来自 `ops_daily_snapshots`；完整度从 definition 2 首次快照起有数据，不回填此前日期。 |
| 数据来源分析 | 现在能算当前分布 | `knowledge_source_records` 按 `knowledge_sources.name/type` 汇总活跃记录数与去重作品数；`work_sources` 按 provider 独立列外部 ID 数与去重作品数，两个口径不相加。来源历史趋势目前没有独立快照字段。 |

作品列表默认只显示至少有一个已发布知识版本的活跃作品。提供「全部活跃目录」筛选后，catalog-only 作品显示「尚无知识版本」，知识计数、验收和完整度返回 NULL，不伪装成已经检查过的零项，也不产生版本完整度结果。

## 5. 接口设计

所有新接口放进现有管理员组，路径前缀为 `/api/v1/admin/knowledge/ops/knowledge/`，继续使用管理员 Bearer token 和 `no-store`。工作台仍沿用 Phase 1 的本机访问与编辑会话门禁，不因此开放新访问范围。

### 5.1 `GET /knowledge/summary`

可选 `range=7d|30d|90d`，返回 `as_of`、口径版本、全库汇总、来源分布和按日趋势：

```json
{
  "as_of": "计算时间",
  "definition": 2,
  "works": { "active_total": 0, "by_type": [], "published_worlds": 0, "release_versions": 0 },
  "people": { "total": 0, "introductions_complete": 0, "relationships": { "availability": "no_data" } },
  "plot": { "storylines": 0, "chapters": 0, "episodes_expected": 0, "episodes_covered": 0, "scenes": 0 },
  "world": { "locations": 0, "organizations": { "availability": "no_data" }, "typed_settings": { "availability": "no_data" } },
  "timeline": { "availability": "no_data", "events": null, "chapter_links": null, "order_checks": null },
  "images": { "main_visuals": 0, "portraits": 0, "chapter_images": 0 },
  "sources": { "records_by_source": [], "external_ids_by_provider": [] },
  "trend": { "range": "30d", "points": [] }
}
```

不适用或无结构字段使用明确的 `availability`，不把 NULL 和 0 混为一类。趋势点携带 `day`、`definition` 和对应指标值；完整度分母为 0 或数据结构缺失时，不返回百分比。

### 5.2 `GET /knowledge/completeness?world=<slug>`

返回某作品当前版本的 `work_id`、`world_slug`、`version_id`、`rule_version`、`calculated_at` 和 `dimensions[]`。每个维度返回 `key`、`completed_required_items`、`applicable_required_items`、`status`、`data_state`、按项状态计数，以及必要的缺项明细。明细项至少包含 `item_code`、`item_key`、`status`、`evidence_ref` 和 `reason`。若版本不存在，返回 `availability='no_data'`，不返回 `0/0` 完整度结果。

### 5.3 作品列表 `GET /knowledge/works`

支持 `q`、`type`、`has_knowledge_version`、`completeness_status`、`production_status`、`sort`、`after`、`limit`。默认 `has_knowledge_version=true`；显式选择「全部活跃目录」时包含 catalog-only。搜索覆盖中外文标题和别名；排序键白名单包括标题、类型、版本数、覆盖集数、人物数、场景数、图片数、完整度、验收通过率和最后更新时间，并用 `work_id` 做稳定的并列排序。

每条 `items[]` 返回：`work_id`、`world_slug`、`title`、`type`、`release_versions`（`world_releases` 行数）、`edition`、`episodes_total`、`episodes_covered`、`characters`、`scenes`、`images`、`completeness`（逐维分项分子 / 分母 / 状态，不含跨实体总分）、`acceptance_pass_rate`、`production_status`、`last_updated_at`、`has_knowledge_version`。另返回 `as_of`、`next_cursor` 和指标定义版本。没有发布版本的目录项，其知识图字段为 NULL 并标记 `has_knowledge_version=false`。作品行验收通过率为每个 topic 最新 `scope_heads` 结果中通过数 / 有最新结果的 topic 数；没有结果时为 NULL。

当前验收通过率的列表口径草案为：按 `scope_heads` 每个 topic 的最新验收结果，`passed=true` 数 / 有最新验收的 topic 数；没有验收记录时为 NULL。分析页按统计期的历史验收通过率仍按审计稿 §5，以结束的 `scope_acceptances` 记录为单位另算，不与当前状态率混用。

### 5.4 单作品详情 `GET /knowledge/works/:work`

`work` 可识别作品 UUID 或唯一 `world_slug`，详情响应包含：

- `overview`：目录字段、当前版本与来源（外部 ID 和活跃来源记录）、全局资产计数、七维完整度分项、缺项与最近质量信息。
- `characters`：人物 slug、姓名、角色、独立介绍、立绘、来源证据；人物关系返回 `availability=no_data`。
- `storylines`：故事线标题、介绍、排序和章节关联。
- `episodes`：作品声明总集数、公开覆盖集、验收 / 待验证状态与章节内容链接。
- `scenes`：场景名、章节、地点、独立文本、证据和图片；无文本的扩展场景明确标缺项。
- `world_settings`：当前可读的地点名称；组织和设定类型明确标无数据。
- `timeline`：结构缺失说明，不构造事件列表。
- `images`：主视觉、人物立绘、章节配图及公开 / 候选状态。
- `quality_report`：`knowledge_quality_issues` 与范围验收摘要。

### 5.5 现有接口复用

| 现有接口 | 当前返回 / 用途 | Phase 2 关系 |
| --- | --- | --- |
| `GET /api/v1/knowledge/works` | 活跃目录分页，已有 UUID、标题、类型、集数、状态、世界链接、封面 | 新管理员列表复用目录过滤与字段来源，新增知识资产、版本、完整度、验收和制作状态字段；不破坏现有响应。 |
| `GET /api/v1/knowledge/works/:id` | 单作品目录、标题、来源、工作室、制作方、标签、外部关系和 provenance | 复用基础档案及来源数据；不把其 `work_relations` 当成人物关系。 |
| `GET /api/v1/knowledge/world-statuses`、`GET /api/v1/knowledge/works/:id/status` | `reader_knowledge_inventory` 的发布版资产计数卡片 | 复用计数定义；不含规则结果、缺项明细和历史趋势。 |
| `GET /api/v1/knowledge/worlds/:work` 及 character / chapter / scene 子路由 | 公开读者版内容页 | 详情页复用这些内容定义；管理员响应额外带来源、验收和质量明细，不改公开接口语义。 |
| Phase 1 `GET /api/v1/admin/knowledge/ops/overview`、`ops/attention` | 运营总览与关注列表 | 与知识汇总分工，不把知识统计塞入原 Overview 响应。 |

## 6. 知识资产界面

- `/workbench/knowledge` 用全库摘要、类型 / 来源分布、按 definition 分段的趋势和可搜索筛选的作品表，替换 Phase 1 复用的 `WorldInventory` 主体。作品行显示 PRD 新字段；catalog-only 行显示「尚无知识版本」，知识计数和完整度显示「—」。
- 单作品详情放在 `/workbench/knowledge/$work`，默认 Overview 汇总与缺项；其它分区为 Characters、Storylines、Episodes、Scenes、World Settings、Timeline、Images、Quality Report。
- `ScopeReport` 放在 Quality Report；`MediaGallery` 放在 Images。沿用 Phase 1 的本机 / 编辑会话检查和 `board.css`，页面只做查看，不新增完整度规则编辑操作。
- Phase 1 的 `?world=<slug>` 旧查询保留；进入详情时解析为该作品的知识详情，详情内部链接改为规范 `$work` 路由，不丢失 `plan`、`task`、`candidate` 等其它上下文参数。

## 7. 日快照与迁移

Phase 0 的 `ops_daily_snapshots` 当前 `definition=1`：全库含活跃作品、类型分布、发布世界和知识资产计数；逐作品含知识资产计数及登记 / 验收 / 发布集数。Phase 2 将新快照升为 definition 2，在全库和逐作品 `metrics` JSON 中新增按 `dimension` 与 `item_code` 分组的完整度：

```json
{
  "completeness": {
    "ruleVersion": 1,
    "dimensions": {
      "people": {
        "status": "partial",
        "items": {
          "introductions": { "completedRequiredItems": 6, "applicableRequiredItems": 8, "status": "partial" },
          "relationships": { "dataState": "no_data", "status": null }
        }
      }
    }
  }
}
```

全库行按维度及 `item_code` 分组汇总所有可计算作品的分子、分母，并统计每个分项的状态作品数及无数据作品数；不同分项不合并。维度状态按每个作品的分项状态推导并统计。逐作品行写该作品各维度的分项分子、分母、状态和规则版本。快照只复制质量扫描已落库的当前完整度结果，不在快照进程里重新评估规则。definition 2 之前的日期不补写完整度，也不回填。

Phase 2 使用 `061.sql`：新增 `knowledge_completeness_rules` 与 `knowledge_completeness_results`、必要索引及迁移维护的规则初始行；不修改业务来源表、不创建关系 / 组织 / 时间线结构。`ops_daily_snapshots.metrics` 已是 JSONB，不需要新增列。质量扫描调度计算并 upsert 规则结果；Phase 0 的 `sync` / `research-lane` 快照进程继续在北京时间 00:05 后把完整度结果写进当天 definition 2 快照。

旧 definition 1 快照不回填、不覆盖。definition 2 趋势从首个 definition 2 快照日期开始；更早日期对完整度留空。趋势按 definition 分段，资产计数也不跨 definition 连线。

## 8. 测试计划

使用隔离 PostgreSQL 和 `apps/api/internal/testsupport/fixtures`，不对开发库做写测试：

1. **规则映射：** 七个维度的正例、缺字段、空白字段、重复项、适用 / 不适用；原创来源不要求外部 `work_sources`，电影没有分集分母，连续剧以声明集数为分母。
2. **完整度状态：** 五种状态的边界；分子仅计完成、分母只计适用项；无关系 / 组织 / Timeline 结构时 `data_state=no_data`，不出现 `0%`。
3. **待验证：** 有 source/capture/research 证据且 latest acceptance 未通过时才是 pending；无证据缺项仍为 missing；通过并发布后转 completed。
4. **版本隔离：** 新 `world_releases.version_id` 产生新结果；重跑同版本幂等更新；旧版结果可查但不冒充当前版。
5. **质量扫描与快照：** 质量扫描落结果且同版本重算幂等；fixture 断言 `cowboy-bebop-tv` 人物介绍 6/8、剧情分项 9/9、10/10、10/26、场景 2/10、图片分项 1/1、8/8、10/10，`tidepost` 人物介绍 0/3、剧情 1/1 和 4/4、场景 2/2、图片分项 1/1、3/3、4/4；definition 1 历史行保留，definition 2 全库与逐作品结果正确且写入幂等、不回填。
6. **API：** 管理员鉴权、分页 / 搜索 / 类型筛选 / 完整度与生产状态筛选 / 稳定排序；作品默认有知识版本、可切换全部活跃目录；详情 UUID 与 slug 解析；`UNKNOWN`、NULL、无数据字段；响应不包含被撤回的私有图片。
7. **业务保护回归：** 现有补集调度、范围验收、发布指针、公开世界详情和图片候选状态测试不变。

本轮按已评审通过的 Phase 2 设计实现迁移 `061.sql`；上线检查运行 `npm run check` 和隔离 PostgreSQL API 测试，开发库不做写入。

## 9. 与审计稿的差异及评审结论

### 差异 / 数据修正

- 审计稿指出人物简介、时间线和分类字段缺失。只读开发库显示 `edition_reader_intros.people` 已给 Cowboy Bebop 的 6/8 个公开人物提供独立简介；另 2 个扩展人物虽有分集出场文字，不等于独立介绍。Tidepost 当前为 0/3。
- 审计稿以源码 / 迁移为依据且未连接制作库；本稿补了本机开发库实际计数，但其迁移最高仅到 057，不能据此声称 Phase 0 日快照或 060 状态码已在该库核验。
- 按产品决定，本阶段仍不创建人物关系、组织 / 设定分类或时间线结构；展示「无数据」，不把结构缺失算作缺失项。

### 已决定事项

1. 剧情不合并故事线、章节、分集的分子 / 分母；图片不合并主视觉、人物立绘、章节配图。其它维度若包含不同实体种类也按分项显示。维度状态由 3.1 定义的分项状态推导，不对分子 / 分母求和。
2. 场景必须有独立场景文本；Cowboy Bebop 为 2/10，另外 8 个扩展分集场景列为缺项。
3. 人物介绍只认可独立人物简介：Tidepost 为 0/3，Cowboy Bebop 为 6/8。
4. 作品表默认仅展示有已发布知识版本的作品；增加「全部活跃目录」筛选。没有知识版本的作品显示「尚无知识版本」，不显示完整度。
5. 版本数使用 `world_releases`；作品行验收通过率按每个 topic 最新结果计算。
6. `UNKNOWN` 类型在类型分布中保留原值，但不算基础档案中的已登记类型；NULL 同样不完成类型项。
7. 趋势按 definition 分段；完整度从 definition 2 开始，之前日期留空、不回填。

当前没有待产品决定的问题。开发库数据限制仍见 2.1：本机库处于 057，不能核验 Phase 0/1 新对象；实现与测试必须使用隔离数据库。
