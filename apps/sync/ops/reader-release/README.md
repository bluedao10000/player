# 单集自动发布

广州准入的分集配置可用 `publishToReader: true` 指定上线。硅谷每 30 秒从 `visible_reader_scopes` 选择一份通过五维验收的单集定稿，读取该集入选 JPEG、角色肖像引用和相关读模型行。通过绑定主机公钥的 SSH 将它们交给阿里云接收程序。接收程序核对作品 UUID、版本、前集和线上肖像。在同一个数据库事务里分配本地 ID、写入公开投影和回执，图片先按摘要持久化，最后提交发布指针。文件已存在时校验其字节，已有章节或场景链接冲突时回滚。

硅谷收到回执后从正式域名验证 H5 段落锚点和返回链接、iOS 章节正文、场景目标及图片摘要。工作台「正式站发布」展示从准入到核验的用时和该集计量 token，按尝试保留错误。发布本身不调用模型。中断或失败的传输等待五分钟重试；每集在接收库有唯一回执。若线上已有不同定稿，保留现场，由运营决定是否走编辑修订流程。

## 首次安装

1. 在仓库根目录运行 `CGO_ENABLED=0 GOARCH=amd64 GOOS=linux go -C apps/sync build -o /tmp/reader-release ./cmd/reader-release`，将二进制安装到硅谷的 `~/folio-data/bin` 和阿里云的 `/opt/codeiy/reader-release`。目录属主分别是调度账号和 root。
2. 广州制作库应用迁移 046 并执行已有的 [媒体角色授权](../production/media-grants.psql)。硅谷沿用其受限 socket 连接。
3. 阿里云读库新建专用登录角色 `folio_release`，运行 [receiver.psql](receiver.psql)。在接收目录以 0600 权限写入 receiver.env，内含该角色连接容器网络 db 的 DATABASE_URL。安装可执行的 [receive.sh](receive.sh)。脚本从运行中的公开 API 查询 archive 挂载位置，容器以归档属主 UID 501 运行。
4. 硅谷用独立 Ed25519 密钥。只把公钥加入阿里云的 authorized_keys，前缀为 `restrict,command="/opt/codeiy/reader-release/receive.sh"`。在硅谷的 `~/folio-data/publish/ssh_config` 定义 Host folio-reader，配置 HostName、User、IdentityFile、IdentitiesOnly、UserKnownHostsFile 和 StrictHostKeyChecking yes。通过管理连接核对服务器主机公钥并写入专用 known_hosts。
5. 在硅谷把同目录下的 service 和 timer 安装到 /etc/systemd/system，执行 `systemctl daemon-reload` 及 `systemctl enable --now folio-reader-release.timer`。发布入口只需要登记 [分集配置](../../../../docs/知识库构建流程.md)。

## 检查与暂停

`systemctl list-timers folio-reader-release.timer` 查看下次运行，`journalctl -u folio-reader-release.service` 查看进程结果。在制作库以 topic 查询 reader_release_attempts，在读库查 reader_release_receipts 可核对两个环境的定稿 ID。暂停新发布使用 `systemctl stop folio-reader-release.timer`，进行中的事务完成后再处理故障。恢复 timer 会重新寻找符合条件的分集。

## 正式站内网传输

硅谷 ssh_config 的 folio-reader 指向阿里云 Mesh 地址，使用 `ProxyCommand /usr/bin/nc -U /srv/folio-data/release-socket/reader.sock`。专用 relay 从硅谷 Mesh IP 连接阿里云 Mesh IP 的 TCP 2222，接收侧转发到本机 sshd。保留原 host key 的 HostKeyAlias 与严格校验。authorized_keys 里的发布专用公钥同时限定阿里云 Mesh Docker 网桥来源、restrict 和原接收命令。发布器只使用这个 socket，公网 HTTPS 用于验证读者看到的结果。地址与规则见[部署架构](../../../../docs/部署架构.md)。
