# 分集生产任务

在已部署的制作环境中，sync 通过共享 PostgreSQL 协调文本和媒体两个任务队列。分节点运行所需的 Pi 和数据库服务定义见 [Compose](compose.yaml)。部署站点和私网见[部署入口](../../../../deploy/README.md)。

1. 从 apps/sync 构建 cmd/knowledge 和 cmd/sync，分别安装为 knowledge 和 knowledge-sync。在服务目录准备私有数据库连接、模型配置、state、media 和 catalog。调度账户、系统路径及挂载点应与本站 systemd 配置一致。
2. 安装 [调度单元](folio-episode@.service)。在每台运行调度器的主机 `/srv/folio-data/lane.env` 中设置唯一 `OPS_NODE_NAME`（如广州 `gz`、硅谷 `sv`），并设置 `APP_VERSION`；两者用于心跳标识和版本记录。文本端加载最近的缺集盘点，按范围、优先级和预算准入，在 Pi 中执行写作与复核。媒体端执行图片生成、自动验图和五维验收。
3. 将 [盘点脚本](inventory.py)、[采集脚本](capture.mjs) 及 [定时器入口](refresh.sh) 安装到其运行路径。盘点进程需要 READER_PUBLIC_URL 和作品版本映射；外部采集模式需要 EPISODE_EXTERNAL_CAPTURE。按需启用 inventory 和 evidence 定时器。
4. 通过 knowledge episode-register 的标准输入登记私有分集计划。在工作台查看缺口、采集失败、制作耗时和验收。只有已配置上线的定稿会进入[正式站投递](../reader-release/README.md)。字段示例见[分集模板](../../episode-plan.example.json)。

盘点定时器每 30 分钟触发，按北京时间在每小时的 00 分和 30 分运行。公开目录失败时，可通过 READER_ORIGIN_CONTAINER 指定已核实的正式读者 API 容器。源站结果要求登记作品齐全，并在快照逐作品标记 reader_origin；公网可用性仍以正式域名读取为准。若盘点节点无法直连 Bangumi，可为盘点单元设置 `BANGUMI_SSH_CONFIG`。该 SSH 配置应有 `bangumi-inventory` 主机项，使用固定主机公钥和单独的密钥。出口节点将对应公钥限制为 `restrict` 和固定命令 [bangumi-page.py](bangumi-page.py)，并限制来源地址。该命令只接受作品 ID 与分页偏移，访问固定的 Bangumi 分集 API。若所有已映射的作品均无法取得分集目录，盘点失败并保留旧快照。

广州用[一次性入队 SQL](enqueue-aired-episodes.sql)从最新完整盘点将这批 20 部的已播缺集写入 PostgreSQL 制作请求，按作品和集号去重。独立的[分批定时器](folio-episode-dispatch.timer)每半小时执行[领取 SQL](dispatch-aired-episodes.sql)一次，每轮最多领取五集。同一时段共用五个名额。缺来源的集从同版本分集目录取得已播集名，通过 OpenAI 在已登记站点查找页面、生成提取规则，并用目标集与参照集验证后挂接。配置失败时将最近一次报错交回生成器。媒体补项按已登记肖像指纹恢复 WebP，已有场景候选继续使用原验图记录。每类恢复最多三轮，用量回执缺失时保留预留等待对账。安装 recover-episodes.py 到服务目录，同步 episode-source-recovery.ts、episode-source-discovery.ts 和 episode-source-rules.ts 到研究 Agent 的 src，并重载研究 Agent 以启用取证规则。节点可用 EPISODE_RECOVERY_ROOT、EPISODE_KNOWLEDGE_BIN、EPISODE_RESEARCH_CONTAINER 和 BANGUMI_SSH_CONFIG 指定路径。人工配置仍可走 `knowledge episode-register`。使用 `journalctl -u folio-episode-dispatch.service` 查看每批领取，使用盘点服务日志或[进度查询](episode-progress.sql)核对缺口和验收。

广州文本 Agent 从权限为 `0600` 的 `research-openai.env` 读取 OpenAI 凭据、模型和计量账户。生产连接使用 [本地 TLS socket](folio-openai-api.socket) 与 [SSH 单连接服务](folio-openai-api@.service)。SSH 客户端固定出口主机公钥。出口侧将专用公钥限制为 `restrict` 和固定的 `/usr/bin/nc api.openai.com 443` 命令，并限制来源地址。容器把 `api.openai.com` 解析到 `127.0.0.2`，仍使用官方域名进行 TLS 证书校验。先从容器核对官方 `/v1/models`，再恢复调度。

```sh
systemctl is-active folio-episode@research
systemctl is-active folio-episode@media folio-refresh@evidence.timer
```

广州媒体节点启用 /state/episode-vision-count.enabled 时，先通过研究节点向 OpenAI 查询本次验图输入的 token 数，再预留输入、硬性输出上限和 128 token 余量。计数服务不可用时使用原有 16,000 token 预留；每集总上限继续由 PostgreSQL 校验。

分别在对应任务节点核验。模型用量、返工及发布回执以工作台记录为准。

`research-lane` 负责分节点部署的北京时间日快照，`media-lane` 不运行快照任务。单服务 Compose 部署由 `sync` 进程负责快照。Agent 通过所在节点调度器每 30 秒轮询 `/internal/status`；在 Agent 的环境配置中设置 `APP_VERSION`，未设置时版本记为 `dev`。

图片步骤的供应商错误记录在 `media_steps.result.provider_error`，用候选号、轮次、步骤和尝试次数定位。记录 UTC 时间、HTTP 状态、接口、模型、请求 ID 和脱敏错误信息。供应商返回审核阶段或类别时一并记录。审核拦截进入 `held`，参数或文件错误进入 `failed`。修正输入并核对参考图摘要后，可用 `knowledge-sync -resume-corrected-image <候选号>` 恢复该步骤。临时限流或服务端错误按 30 秒、60 秒退避，单步骤最多尝试三次。用量待对账时保留预留，暂停新的付费调用。
