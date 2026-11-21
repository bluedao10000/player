# 分集生产任务

在已部署的制作环境中，sync 通过共享 PostgreSQL 协调文本和媒体两个任务队列。分节点运行所需的 Pi 和数据库服务定义见 [Compose](compose.yaml)。部署站点和私网见[部署入口](../../../../deploy/README.md)。

1. 从 apps/sync 构建 cmd/knowledge 和 cmd/sync，分别安装为 knowledge 和 knowledge-sync。在服务目录准备私有数据库连接、模型配置、state、media 和 catalog。调度账户、系统路径及挂载点应与本站 systemd 配置一致。
2. 安装 [调度单元](folio-episode@.service)。文本端加载最近的缺集盘点，按范围、优先级和预算准入，在 Pi 中执行写作与复核。媒体端执行图片生成、自动验图和五维验收。
3. 将 [盘点脚本](inventory.py)、[采集脚本](capture.mjs) 及 [定时器入口](refresh.sh) 安装到其运行路径。盘点进程需要 READER_PUBLIC_URL 和作品版本映射；外部采集模式需要 EPISODE_EXTERNAL_CAPTURE。按需启用 inventory 和 evidence 定时器。
4. 通过 knowledge episode-register 的标准输入登记私有分集计划。在工作台查看缺口、采集失败、制作耗时和验收。只有已配置上线的定稿会进入[正式站投递](../reader-release/README.md)。字段示例见[分集模板](../../episode-plan.example.json)。

```sh
systemctl is-active folio-episode@research
systemctl is-active folio-episode@media folio-refresh@evidence.timer
```

分别在对应任务节点核验。模型用量、返工及发布回执以工作台记录为准。
