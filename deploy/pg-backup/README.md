# 广州制作库每日备份

K3s 的 `folio-pg-backup` 命名空间独立运行 PostgreSQL 17。CronJob 每天北京时间 03:15 从广州的 `folio` 业务库获取一致性逻辑快照。备份机通过受限的本机 Unix socket 和 Cloudflare Mesh 连接广州 TCP 5433。数据库凭据分别保存在 Kubernetes Secret，每个节点单独注册 Mesh。[Mesh 服务](../first-phase/mesh/operations.yaml)

## 配置与验收

1. 核对节点标签、默认 local-path 存储类、两份 16 GiB PVC 的可用磁盘和镜像摘要。离线安装可从已有广州节点导出 linux/amd64 镜像，导入 K3s containerd 并核对平台 manifest。备份节点上的 Docker 使用核对 config digest 后的本地 Mesh 镜像。[离线覆盖文件](../first-phase/mesh/backup-offline.yaml)
2. 在广州建立专用普通登录角色 `folio_backup_read`，应用 [只读授权](source-grants.sql)。当前业务对象都在 public，由 folio 持有；增加 schema 或对象属主时，同步授予备份读取权限。新建 namespace 和两个 Secret，postgres-login 保存 target-password，guangzhou-login 保存 source-password。源库密码仅备份 Job 挂载。
3. 在备份节点创建仅 UID 70 可访问的 /var/lib/folio-mesh/backup-socket。启动广州 backup-listener 与备份节点 backup-socket，核对 Gateway 仅允许备份节点到广州指定端口。执行 `kubectl apply -k deploy/pg-backup`。
4. 使用 `kubectl -n folio-pg-backup create job --from=cronjob/guangzhou-backup backup-check-$(date +%s)` 立即演练。用 `kubectl -n folio-pg-backup get jobs,pvc` 及 Job 日志查看结果。在 postgres-0 中使用 `psql -U postgres -d folio_backup` 检查恢复库。

pg_dump 与逐表计数使用同一导出快照。任务写入独立快照卷，计算 SHA-256，在新数据库用单事务恢复，并逐表比对记录数。验证通过后原子切换 folio_backup 库名，上一版保留为 folio_previous，再保留最近七份成功归档。定时器禁止并发，卷文件锁也覆盖手动任务。快照目录以 UTC 命名，失败任务保留 Kubernetes 状态和日志。每日任务最多重试四次，执行窗口三小时。

备份覆盖业务数据库中的表、序列、视图、函数、索引和数据。恢复演练在独立 PostgreSQL 中使用本地属主；重建生产权限时另行配置集群角色。硅谷图片文件、Pi 执行卷和阿里云读者数据各有独立存储，本任务仅覆盖广州数据库。快照与恢复库分卷，底层均在 192.168.1.7 的本地磁盘。
