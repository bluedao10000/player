# 分集生产任务

在已部署的制作环境中，sync 通过共享 PostgreSQL 协调文本和媒体两个任务队列。分节点运行所需的 Pi 和数据库服务定义见 [Compose](compose.yaml)。部署站点和私网见[部署入口](../../../../deploy/README.md)。

1. 从 apps/sync 构建 cmd/knowledge 和 cmd/sync，分别安装为 knowledge 和 knowledge-sync。在服务目录准备私有数据库连接、模型配置、state、media 和 catalog。调度账户、系统路径及挂载点应与本站 systemd 配置一致。
2. 安装 [调度单元](folio-episode@.service)。在每台运行调度器的主机 `/srv/folio-data/lane.env` 中设置唯一 `OPS_NODE_NAME`（如广州 `gz`、硅谷 `sv`），并设置 `APP_VERSION`；两者用于心跳标识和版本记录。文本端加载最近的缺集盘点，按范围、优先级和预算准入，在 Pi 中执行写作与复核。媒体端执行图片生成、自动验图和五维验收。
3. 将 [盘点脚本](inventory.py)、[采集脚本](capture.mjs) 及 [定时器入口](refresh.sh) 安装到其运行路径。盘点进程需要 READER_PUBLIC_URL 和作品版本映射；外部采集模式需要 EPISODE_EXTERNAL_CAPTURE。按需启用 inventory 和 evidence 定时器。
4. 通过 knowledge episode-register 的标准输入登记私有分集计划。在工作台查看缺口、采集失败、制作耗时和验收。只有已配置上线的定稿会进入[正式站投递](../reader-release/README.md)。字段示例见[分集模板](../../episode-plan.example.json)。

盘点定时器每 30 分钟触发，按北京时间在每小时的 00 分和 30 分运行。若盘点节点无法直连 Bangumi，可为盘点单元设置 `BANGUMI_SSH_CONFIG`。该 SSH 配置应有 `bangumi-inventory` 主机项，使用固定主机公钥和单独的密钥。出口节点将对应公钥限制为 `restrict` 和固定命令 [bangumi-page.py](bangumi-page.py)，并限制来源地址。该命令只接受作品 ID 与分页偏移，访问固定的 Bangumi 分集 API。若所有已映射的作品均无法取得分集目录，盘点失败并保留旧快照。

广州用[一次性入队 SQL](enqueue-aired-episodes.sql)从最新完整盘点将这批 20 部的已播缺集写入 PostgreSQL 制作请求，按作品和集号去重。独立的[分批定时器](folio-episode-dispatch.timer)每半小时执行[领取 SQL](dispatch-aired-episodes.sql)一次，每轮最多领取五集。来源模板不足的请求保留卡点，补齐模板后可由后续批次领取。通过人工登记补齐且已验收的集会同步到队列。人工配置仍可走 `knowledge episode-register`。使用 `journalctl -u folio-episode-dispatch.service` 查看每批领取，使用盘点服务日志或[进度查询](episode-progress.sql)核对缺口和验收。

广州文本 Agent 从权限为 `0600` 的 `research-openai.env` 读取 OpenAI 凭据、模型和计量账户。生产连接使用 [本地 TLS socket](folio-openai-api.socket) 与 [SSH 单连接服务](folio-openai-api@.service)。SSH 客户端固定出口主机公钥。出口侧将专用公钥限制为 `restrict` 和固定的 `/usr/bin/nc api.openai.com 443` 命令，并限制来源地址。容器把 `api.openai.com` 解析到 `127.0.0.2`，仍使用官方域名进行 TLS 证书校验。先从容器核对官方 `/v1/models`，再恢复调度。

```sh
systemctl is-active folio-episode@research
systemctl is-active folio-episode@media folio-refresh@evidence.timer
```

分别在对应任务节点核验。模型用量、返工及发布回执以工作台记录为准。

`research-lane` 负责分节点部署的北京时间日快照，`media-lane` 不运行快照任务。单服务 Compose 部署由 `sync` 进程负责快照。Agent 通过所在节点调度器每 30 秒轮询 `/internal/status`；在 Agent 的环境配置中设置 `APP_VERSION`，未设置时版本记为 `dev`。
