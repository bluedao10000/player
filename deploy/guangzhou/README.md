# 广州业务主节点

此目录定义广州业务 Compose 项目。Nginx 从专用私网访问 Gin、Reader API 和图片库。读者库、制作库和图片卷各自管理。

## 配置与状态

将 [.env.example](.env.example) 复制到私有 /etc/player-gz/settings。准备 reader.env、sync.env、release.env、sender.env、agent.env、image.env，权限 0600；容器仅接收自己的文件。所有数据卷必须按实际名称显式指定为 external。隔离验收使用从备份恢复的独立卷和测试凭据。已有数据卷不可同时交给两个 PostgreSQL、Pi 或 Image 写进程。

| 服务 | 输入、身份与持久状态 |
|---|---|
| postgres:5432 | PG17 专用数据卷；管理密码仅用于初始化。外部卷已有集群时沿用其实际管理员。以整理后的原角色/grants 恢复两个库，各自拥有者负责 DDL |
| api:4311 | PGHOST=postgres，READER_DB、READER_USER 与 reader.env 的 PGPASSWORD。归档 /archive 只读；原微信回调、支付凭据、邮件、StoreKit 环境单独放 reader.env。关闭工作台本地登录。/healthz 会 ping 数据库 |
| reader-api:4310 | KNOWLEDGE_API_URL=http://api:4311。由 READER_API_TRUSTED_PROXY_IP 指定广州 Mesh 连接器的固定网桥地址，透传公开站 Host 和 HTTPS。提供登录、iOS、支付与 sitemap |
| image:14580 | 继承 apps/image 的构建、资源限制、只读根文件系统和健康检查，挂载独立本地 /data；删除继承的宿主机映射。image.env 恢复原签名密钥及管理 token。默认关闭资产库 AI 领取，读、上传、签发和预热仍可用 |
| research-agent:4312 | Pi 的 /state（含 agent.sqlite、model-usage.sqlite 及关联文件）、/media 和参考图。agent.env 配 OpenAI 的 CONTENT_AI_API_KEY、CONTENT_AI_MODEL 和 CONTENT_AI_ACCOUNT；通过验证过的 OpenAI HTTPS 路径访问。MEDIA、VISION 的模型、ACCOUNT 沿用原计量映射，API_KEY 为硅谷网关身份。两者 BASE_URL 指向同命名空间的 model-egress |
| model-egress:18080 | 仅 Pi 回环可访问。UID 1000 读取 MODEL_TLS_DIR 中的 ca.pem、client.pem、client.key，经 MODEL_SOCKET_DIR/model.sock 连接硅谷，校验 SV_TLS_NAME 并提交客户端证书。证书目录只读；文件系统仅允许专用服务身份读取私钥 |
| sync、sync-media | 与原生产单元相同的 research/media 两条 lane，连接 PRODUCTION_DB、PRODUCTION_USER，密码在 sync.env。各自容器内可监听预算端口 14328，Pi 使用 sync:14328。制作归档、计划和 /media 与读者归档分卷。两条 lane 与 Pi 均在 workers profile |
| release-receiver | 明确授权时临时运行。release.env 内 DATABASE_URL 指向 postgres 上的读者库，专用发布角色具有 receiver.psql 所需权限；接收器写读者归档，使用 IMAGE_API_TOKEN 访问私有图片管理接口 |

广州宿主机按服务限制出口，Reader API 只需历史 Folio 通知等既有目标；Gin 保留实际启用的认证/支付供应商；Agent 仅允许国内网关、必要证据源和受控模型中继。禁止广州直连境外模型域名。治理端另设内网服务身份；公开 API 不接收治理、模型管理员或制作库凭据。暂不向 image 注入任何境外供应商凭据；若启用图片 AI，应先给它单独的网关身份和同等 mTLS 出口。卷内对象键及 SQLite 目录保留，未来 S3/R2 适配沿用 [图片存储契约](../../apps/image/README.md)。

## 启动顺序与验收

以下命令从仓库根目录运行，先在隔离项目演练。

```sh
gz() { docker compose --env-file /etc/player-gz/settings -f deploy/guangzhou/compose.yaml "$@"; }
gz --profile '*' config --quiet
gz build reader-api api image research-agent sync sync-media release-sender release-receiver
gz up -d --wait postgres
```

1. 创建专用 Docker 网桥，配置经核实的 subnet 和动态分配范围，**为 GZ_EDGE_BRIDGE_IP 预留动态分配范围以外的地址**。预先建立或还原所有 external 卷。镜像使用宿主机匹配的架构。恢复 PostgreSQL、ACL、签名材料、源图片与读者归档，检查卷属主（Image UID 10001，Go 服务 UID 65534，Agent UID 1000）。归档目录应可被接收器写入。恢复时隔离新库，禁止它发真实通知和启动任务。
2. 对两个数据库分别撤销 PUBLIC CONNECT，单独授予各库运行、迁移、备份、发布身份 CONNECT。禁止运行身份 SUPERUSER、CREATEDB、CREATEROLE 或继承另一库角色。按实际需要授予命名表、序列和函数权限。恢复应用本次版本前的 ACL，并用普通身份实测。专用迁移身份在隔离网络运行同镜像 api，设置 PGHOST、目标 PGDATABASE、DDL 用户和 KNOWLEDGE_VERIFY_SCHEMA_ONLY=false；/healthz 成功即退出该临时进程，再以另一个库的迁移身份重复。运行版 api/sync 固定 verify-only。分别查 knowledge_migrations，与构建版本比对；用两个应用账号互连对方库须拒绝。
3. `gz up -d --wait api image reader-api`。检查 Gin /healthz、Reader API /healthz 与图片 /readyz。从网关验证公开路由、读者认证 Origin/Cookie、iOS API 和支付通知。
4. 按[网关手册](../gateway/README.md)导入和核验在用图片原件，随后启用 img.codeiy.com 与静态站点。
5. **生产单写闸门通过后**，排空旧广州和硅谷的研究、媒体、发布与 refresh 定时器。对每个 Pi 运行、未确认供应商请求、outbox event_id、制作任务 ID 及投递回执建立清单。正常关闭 SQLite 后复制整套检查点和原图；两端有独立 SQLite 时先各自恢复并完成在途和 outbox 对账，选择一个已排空的状态卷启动。禁止拼接 SQLite 或用一端覆盖另一端未完成状态。核实预算地址与账户映射。
6. 启动 `gz --profile workers up -d --wait research-agent model-egress`，在其命名空间验证 mTLS 正反例、授权模型列表和预算配置。再运行 `gz --profile workers up -d sync sync-media`，观察日志、一次批准的研究/媒体运行、计量回执和唯一发布指针；容器 running 本身不能证明正在正确领取。迁移原 refresh 定时器与输入文件。切换期间仅运行一套定时器/每条 lane 的一个领取者。
7. 安装本目录 [接收包装脚本](receive.sh) 为不可由 SSH 登录身份修改的 forced command，固定 /opt/player 检出与 /etc/player-gz/settings。执行 Docker 的权限等同宿主管理权限，限制为固定命令（专用 restrict 密钥、禁 shell/端口转发，不给通用 Docker socket）。原 receiver 固定查找阿里云容器名和网络，不能直接复制。广州沿用[既有 timer](../../apps/sync/ops/reader-release/folio-reader-release.timer)，将配对 service 换成[广州发送单元](folio-reader-release.service)。sender.env 只放制作库发布身份 DATABASE_URL 与 READER_PUBLIC_URL；发送容器通过 Docker 网络读制作库及只读原件。PUBLISH_SSH_DIR 内的 ssh_config 以 folio-reader 为别名，HostName 指向仅在私有网桥开放的广州接收 sshd，可使用 host.docker.internal 和实际监听端口。指定 /publish 下的 IdentityFile 与 UserKnownHostsFile、IdentitiesOnly yes、StrictHostKeyChecking yes，私钥仅由发送容器 UID 65534 可读。用一次已授权定稿验证段落、iOS、图片 SHA、同定稿重复投递只产生原回执，再启用定时器。
8. 依照[网络矩阵](../../docs/部署架构.md)接入 PostgreSQL 与图片备份。分别还原并核对发布回执、图片 SHA-256、读者订单和用量。
