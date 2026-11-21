# Cloudflare 入口与私网

公网域名经 Cloudflare 边缘和 Tunnel 到入口网关。网关将 H5 与公开 API 转给站点服务，将已登记的图片路径经 Mesh 中继转给图片服务。制作节点和备份节点使用受限的私网连接访问数据库；发布任务通过专用 SSH 命令投递已验收内容。

| 部件 | 安装与验证 |
|---|---|
| 公网 Tunnel | 私下配置域名、入口和注册令牌，将令牌作为 systemd credential 交给 [tunnel 单元](player-tunnel.service)。启动后从公网验证站点。 |
| HTTP 网关 | 在 [Nginx 模板](player-ingress.conf) 中填写实际域名和本机上游。启用图片分流时使用 [sync 导入工具](../../apps/sync/ops/image-import/import.py) 生成限定资源路径的配置。用 nginx -t 检查后加载。 |
| 节点 Mesh | 每台节点独立配置 mesh-node.env 和注册状态卷，从 [基础 Compose](mesh/compose.yaml) 启动 mesh，再配置所需的节点地址、/32 路由和 Gateway 策略。 |
| 私网服务 | 按角色叠加 [发布和备份中继](mesh/operations.yaml) 或 [图片中继](mesh/images.yaml)。使用 [配置脚本](mesh/configure-peer.py) 时通过环境提供管理凭据，仅放行指定源、目标和端口。 |

在节点的私有安装目录中验证合并后的 Compose，然后启动已配置的角色。例如，发布和备份中继使用以下命令。ROLE 对应模板里的 profile 名称。

```sh
docker compose -f compose.yaml -f operations.yaml --profile "$ROLE" config --quiet
docker compose -f compose.yaml -f operations.yaml --profile "$ROLE" up -d
```

数据库验证用受限身份进行只读查询。图片分流分别核对公开原件摘要和预览鉴权。定稿投递按[发布手册](../../apps/sync/ops/reader-release/README.md)核对目标回执，再从公开入口验证章节和段落锚点。
