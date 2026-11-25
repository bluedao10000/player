# 广州制作库每日备份

K3s 的 `folio-pg-backup` 命名空间独立运行 PostgreSQL 17。CronJob 每天北京时间 03:15 从广州的 `folio` 业务库获取一致性逻辑快照。备份机通过受限的本机 Unix socket 和 Cloudflare Mesh 连接广州 TCP 5433。数据库凭据分别保存在 Kubernetes Secret，每个节点单独注册 Mesh。[Mesh 服务](../cloudflare/mesh/operations.yaml)

## 配置与验收

1. 核对节点标签、默认 local-path 存储类、两份 16 GiB PVC 的可用磁盘和镜像摘要。离线安装可从已有广州节点导出 linux/amd64 镜像，导入 K3s containerd 并核对平台 manifest。备份节点上的 Docker 使用核对 config digest 后的本地 Mesh 镜像。[离线覆盖文件](../cloudflare/mesh/backup-offline.yaml)
2. 在广州建立专用普通登录角色 `folio_backup_read`，应用 [只读授权](source-grants.psql)。当前业务对象都在 public，由 folio 持有；增加 schema 或对象属主时，同步授予备份读取权限。新建 namespace 和两个 Secret，postgres-login 保存 target-password，guangzhou-login 保存 source-password。源库密码仅备份 Job 挂载。
3. 在备份节点创建仅 UID 70 可访问的 /var/lib/folio-mesh/backup-socket。启动广州 backup-listener 与备份节点 backup-socket，核对 Gateway 仅允许备份节点到广州指定端口。执行 `kubectl apply -k deploy/pg-backup`。
4. 使用 `kubectl -n folio-pg-backup create job --from=cronjob/guangzhou-backup backup-check-$(date +%s)` 立即演练。用 `kubectl -n folio-pg-backup get jobs,pvc` 及 Job 日志查看结果。在 postgres-0 中使用 `psql -U postgres -d folio_backup` 检查恢复库。

pg_dump 与逐表计数使用同一导出快照。任务写入独立快照卷，计算 SHA-256，在新数据库用单事务恢复，并逐表比对记录数。验证通过后原子切换 folio_backup 库名，上一版保留为 folio_previous，再保留最近七份成功归档。定时器禁止并发，卷文件锁也覆盖手动任务。快照目录以 UTC 命名，失败任务保留 Kubernetes 状态和日志。每日任务最多重试四次，执行窗口三小时。

备份覆盖业务数据库中的表、序列、视图、函数、索引和数据。恢复演练在独立 PostgreSQL 中使用本地属主；重建生产权限时另行配置集群角色。硅谷图片文件、Pi 执行卷和阿里云读者数据各有独立存储，本任务仅覆盖广州数据库。快照与恢复库分卷，底层均在 BACKUP_LAN_HOST 的本地磁盘。

## 备份回执

`backup-report` 每三分钟从本命名空间读取 `guangzhou-backup` 的 Job 与容器结果，以 Kubernetes Job UID 写入广州制作库的 `backup_runs`。执行明细根据 Job 注解区分定时与手动触发，同一 Job 的 Kubernetes 重试算作一次执行。每个失败容器的结束时间与原因独立保存，后续重试成功仍可展开查看。新备份完成归档校验、独立库恢复、行数比对和切换后，由备份容器写入结构化回执。旧归档可在核对 Job 日志中的快照目录、校验和及两份行数清单后，将实测结果写入对应 Job 的 `folio.codeiy.com/verified-archive` 注解。核对成功 Job 时，同时检查是否收到校验回执。

采集心跳随记录一起提交。可用心跳时间核对采集延迟。Mesh 或广州数据库中断时，采集器恢复后会补传 K3s 仍保留的 Job。若中断跨越 Job 的保留窗口，已被 Kubernetes 清理且未曾采集的执行需用节点归档单独核查。采集器只读取日志末尾并归类失败原因，原始日志保留在 Kubernetes。

### 部署采集器

1. 在广州应用迁移 `047.sql` 至 `049.sql`，创建普通登录角色 `folio_backup_report`，使用独立随机密码并应用 [回执表授权](report-grants.psql)。将同一密码保存在命名空间 Secret `backup-report-login` 的 `password` 字段。备份读取与回执写入使用各自凭据。
2. 在仓库根目录构建节点架构对应的二进制，把二进制和 [Dockerfile](Reporter.Dockerfile) 复制到节点上的独立构建目录。当前节点为 linux/amd64。

   ```sh
   CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go -C apps/sync build -trimpath -o /tmp/backup-report ./cmd/backup-report
   ```

3. 在节点构建目录制作镜像并导入 K3s。改版时同步更新镜像标签与 [CronJob](reporter.yaml) 的 image 字段。

   ```sh
   sudo docker build --network none -f Reporter.Dockerfile -t localhost/folio/backup-report:v1.3 .
   sudo docker save localhost/folio/backup-report:v1.3 | sudo k3s ctr images import -
   kubectl apply -k ~/folio-data/pg-backup
   kubectl -n folio-pg-backup create job --from=cronjob/backup-report report-check-$(date +%s)
   ```

4. 用 `kubectl -n folio-pg-backup get jobs` 核对采集任务，在广州制作库读取 `backup_runs` 并比对任务及校验时间。

## 广州图片包与读者库

读者库复用 backup.sh 的一致快照流程，显式配置 SOURCE_DATABASE 和 SOURCE_USER，使用独立 namespace、目标 PostgreSQL 和 PVC。制作库原有定时任务保持自己的身份与恢复库。

1. 广州按[图片手册](../../apps/image/README.md#备份与恢复)在私有暂存区生成新包。目录名使用 UTC 的 image-YYYYMMDDTHHMMSSZ；确认 COMPLETE 后在同一文件系统将目录 rename 到只读导出根，将仅含该目录名的临时文件 rename 为 LATEST。保留拉取窗口内的源包。广州先验证 rrsync 及其脚本解释器可执行。sshd 的专用受限密钥固定执行 rrsync -ro 导出根，强制 restrict 和对端来源，用只读 Unix 身份运行。导出目录只包含完整包和 LATEST。
2. 在广州和 K3s 的现有 Mesh 项目叠加[图片备份中继](../gateway/mesh/image-backup.yaml)，只启用本节点对应服务。明确填写 GZ_PRIVATE_SSH_IP、GZ_PRIVATE_SSH_PORT（从广州 mesh 命名空间可达的私有 sshd）、双方已登记 /32 和 GZ_IMAGE_BACKUP_PORT。K3s 的 IMAGE_BACKUP_SOCKET_DIR 由 UID 10001 独占，主机上先创建该目录。只放行 BACKUP_MESH_IP 到广州此端口。
3. 从仓库根目录构建备份镜像：`docker build --build-arg IMAGE_RUNTIME="$VERIFIED_IMAGE_RUNTIME" -t "$IMAGE_BACKUP_IMAGE" deploy/pg-backup/image`。使用与生产一致的已验收图片运行时，归档构建后镜像摘要。准备独立 PVC image-snapshots，容量覆盖七份成功包、进行中的第八份及恢复临时空间；Secret image-backup-ssh 含 config、专用密钥、known_hosts。config 的 Host source 使用 ProxyCommand /usr/bin/nc -U /mesh/image-backup.sock、HostKeyAlias、StrictHostKeyChecking yes、IdentityFile、UserKnownHostsFile、IdentitiesOnly yes 和专用只读 User。通过独立途径核对主机公钥。
4. 对 [CronJob 模板](image/cronjob.yaml.in) 仅替换 BACKUP_NODE、IMAGE_BACKUP_IMAGE、IMAGE_BACKUP_SOCKET_DIR，先在独立命名空间验证配置，再创建一次手动作业验收。每日作业使用同一快照校验对象清单和所有摘要，启动恢复的 Axum 实例，以临时签名密钥读取样本资产并比对 SHA-256，通过后才纳入轮换。SSH、传输、校验或恢复失败时保留上次成功包。旧预览 key、旧 library 和生产签名密钥独立备份，另行定期从受限保管位置恢复并验证既有签名。[拉取与恢复脚本](image/pull-verify.sh)
