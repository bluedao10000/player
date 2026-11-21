# 腾讯云分集制作

广州运行知识库写库、Magpie 和研究 Pi。硅谷运行 OpenAI 媒体 Pi、媒体调度器和五维验收。两个调度器共用一个工作库。生产 H5 与 iOS 继续通过阿里云的公开读模型访问定稿。

| 节点 | 服务 | 持久化与连接 |
| --- | --- | --- |
| 广州 | Docker PostgreSQL 17、folio-pi-research、folio-episode@research | PostgreSQL 绑定 127.0.0.1:15432，Pi 使用本机 Magpie 3425 端口。知识库位于 folio-knowledge-pg17 卷。 |
| 硅谷 | folio-pi-image、folio-episode@media、folio-refresh 定时器 | Pi 使用本机图像和视觉审核凭据。调度器经已配置的专用数据库连接读选题、写回执。 |
| 阿里云 | Gin 公开投影、H5、iOS 读接口 | 逐集同步已通过验收的定稿和对应的内容寻址图片。 |

## 部署

服务根目录是两台主机上的 /home/ubuntu/folio-data。compose.yaml 的两个 profile 分别描述两台主机的容器。镜像使用 apps/agent/Dockerfile，agent-src 固定到与调度器一致的源码版本。Go 二进制分别由 apps/sync/cmd/knowledge 和 apps/sync/cmd/sync 编译到 bin/。在仓库根执行 `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go -C apps/sync build -o /tmp/knowledge-sync ./cmd/sync` 可生成 systemd 使用的调度器。state、media 持久化并允许容器内的 node 用户写入。数据库和模型配置文件权限为 0600。

广州首次部署生成独立的数据库管理员口令和知识库所有者 folio 的登录口令。postgres.env 供 Docker 初始化实例，database.env 供调度器读取 DATABASE_URL。恢复经过筛选的作品知识库、验图凭证与前三集定稿。episode-lane 不执行读者订单、编辑队列或其他来源的全量清洗。lane.env 在广州设置 EPISODE_EXTERNAL_CAPTURE=true。

广州用 `bin/knowledge episode-register < plan.json` 登记作品范围和分集资料。执行环境从 database.env 读取凭据。登记事务同时校验作品版本、锁定分集正文定位和路由、登记美术设定及准入策略。只调整策略时可省略 episodes，仍需提供作品、provider、externalID、年份及类型。硅谷的采集进程通过数据库获取已锁定的完整配置。catalog/ 用于每日盘点的版本映射。

广州 research 实例读取最近 48 小时的缺集快照，按优先级从高到低、再按季内集号准入，每次准入在同一事务中计入作品集数与 token 预留。硅谷启用 folio-refresh@inventory.timer（北京时间 06:00）和 folio-refresh@evidence.timer（上次运行结束后 5 分钟）。采集租约 10 分钟，失败至少间隔 15 分钟再试。一集来源失败后继续处理其他可领取集。广州 Pi 从已核验快照取证并经 Magpie 写作；硅谷 media 实例通过 Pi 直连 OpenAI 出图、复核，Go 在原图所在节点验收。媒体候选图、原图、裁切图和历史肖像位于硅谷 media/；五维验收直接核对该节点的字节与数据库中的审核记录。

## 核验

- 通过各自回环地址的 /healthz 检查 Pi，再实调配置好的具体模型。Magpie 的 /v1/models 仅表示路由已登记。
- 在广州用工作库查询 episode_inventory_snapshots、episode_source_captures、research_runs 和 model_calls。检查 scope_heads.published、入选图文件摘要后才能同步阿里云。
- 比对生产 H5 的章节和场景锚点，以及 iOS 公开章节 JSON 中的正文、场景与插图摘要。第 4 集的验收基线见 docs/运行记录/2026-11-21-作品逐集缺口.md。

## Mesh 激活与故障定位

Mesh 节点使用 folio-guangzhou-knowledge 和 folio-siliconvalley-media。mesh/compose.yaml 通过逐机令牌建立两个独立节点。先单独启动 mesh 服务。主机挂载 /dev/net/tun，容器独享网络命名空间与注册状态卷。广州 PostgreSQL 已接入专用 Docker 网络 folio-mesh-transport，数据库别名为 folio-db。硅谷的 socket 目录仅允许调度用户访问。

1. 在账户侧检查两个节点的注册状态，在容器内运行 warp-cli status，并从网络接口取得两端实际的 Mesh IPv4。记录节点名称和地址的对应关系。注册失败时保留原节点和状态卷。
2. 给这两个节点的配置启用 Mesh 对端路由，确认 Gateway 的 TCP 转发，以及硅谷节点到广州节点 5432 的专用放行规则。检查相关配置对既有设备的作用域。两端 mesh 目录中的 Compose 变量 GZ_MESH_IP、SV_MESH_IP 必须填写实测值。
3. 广州启用 guangzhou profile 的 postgres-listener，硅谷启用 silicon-valley profile 的 worker-socket。先在硅谷以专用身份从 socket 查询当前库和当前用户，再运行 knowledge episode-evidence-due。该查询还会检查服务需要的数据库迁移。仅看到节点在线或 TCP 建连不能完成这一步验收。
4. 确认 SQL 成功后，硅谷才启用 database.mesh.env 作为 database.env，启动 folio-episode@media、folio-refresh@evidence.timer，并运行 folio-refresh@inventory.service 补录积压。核对第 4 集生产回执和五维验收状态。需要停用时先停硅谷媒体调度，再停端口转发容器，保留工作库和执行卷。

Cloudflare API 的 WARP Write 管理节点与注册令牌。Zero Trust 组织开通、客户端设备配置和 Gateway 网络规则是另外的账户资源。3007 出现在节点主机向 Cloudflare 注册阶段；403 出现在管理令牌读取账户设置阶段。排查时分别记录两次请求，使用脱敏错误摘要。

部署验收应检查运行中容器的挂载和文件内容。硅谷执行以下命令，两份摘要应一致，健康检查和取证定时器均应正常。旧容器可能在服务器文件更新后仍运行镜像内的旧脚本。[两部作品的补集验收](../../docs/运行记录/2026-11-21-配置式补集验收.md)

```sh
cd ~/folio-data
sha256sum agent-src/media.ts
sudo docker exec folio-pi-image sha256sum /app/apps/agent/src/media.ts
curl -fsS http://127.0.0.1:4312/healthz
systemctl is-active folio-episode@media folio-refresh@evidence.timer
systemctl is-enabled folio-refresh@evidence.timer
```

### 四节点 Mesh 的服务配置

在每台机器的 mesh 目录安装 compose.yaml 和 operations.yaml，在 .env 中设置 COMPOSE_FILE 为 `compose.yaml:operations.yaml` 及对应的 COMPOSE_PROFILES。四个 profile 分别为 guangzhou、silicon-valley、aliyun、backup。局域网离线 Docker 额外在 COMPOSE_FILE 末尾追加 `:backup-offline.yaml`。各节点使用自己独立的 mesh-node.env 和 registration 卷。

广州配置 GZ_MESH_IP、SV_MESH_IP、BACKUP_MESH_IP；硅谷配置 GZ_MESH_IP、ALI_MESH_IP 并创建仅调度用户可访问的 release-socket 目录；阿里云配置 ALI_MESH_IP、SV_MESH_IP、ALI_DOCKER_GATEWAY。备份节点配置 GZ_MESH_IP 及 UID 70 专属 socket 目录。地址以各节点实际注册结果为准，ALI_DOCKER_GATEWAY 取专用网桥的 IPv4 网关。用 `docker compose config --services` 核对服务清单。

在管理端装入 Cloudflare 账户环境后，运行下列命令增量加入两个端口与 /32 路由。保留既有媒体连接许可和私网默认拒绝策略。每台机器在自己的 profile 下运行 `docker compose up -d`；轮换 mesh 容器时连同共享其网络命名空间的 relay 一起重建。按[发布手册](../reader-release/README.md)和[备份手册](../pg-backup/README.md)验证业务结果。

```sh
python3 deploy/first-phase/mesh/configure-peer.py release 100.96.0.2 100.96.0.3
python3 deploy/first-phase/mesh/configure-peer.py backup 100.96.0.4 100.96.0.1
```
