# 新环境部署

从仓库根目录运行以下命令。基础 Compose 启动 PostgreSQL、Gin API、sync 和 Pi Durable，公开 H5 另由 Nginx 托管。首次部署需要 Docker Compose v2.20.3 以上及 Node 24.13+。数据库、媒体归档和 Agent 检查点使用命名卷。

部署时必须在 .env 中显式设置唯一 OPS_NODE_NAME（例如单机生产用 prod、广州用 gz、硅谷用 sv）；deploy/docker-compose.yml 保留必填校验，缺失或空值会阻止启动。根目录 compose.yaml 仅用于本地开发，缺失或空的 OPS_NODE_NAME 默认 local，无需 compose.local-env。两个入口共用 compose.services.yml，根入口通过 compose.local.yml 配置本地默认值。分节点 systemd 调度在每台主机的 lane.env 中显式设置该节点唯一值和 APP_VERSION，见[生产调度说明](../apps/sync/ops/production/README.md)。

作品世界目录导入或分集登记新建作品时，会在同一事务内自动补齐已有 `mio`、`bebop%` 图片候选的批次归属；重复导入不会重复创建映射。

```sh
test -f .env || (umask 077 && cp deploy/.env-example .env)
# 在 .env 中设置随机 POSTGRES_PASSWORD 和非空且唯一的 OPS_NODE_NAME；生产不可使用 local
docker compose --env-file .env -f deploy/docker-compose.yml config --quiet
docker compose --env-file .env -f deploy/docker-compose.yml up -d --build
curl -fsS http://127.0.0.1:4321/healthz
npm ci
npm run dev:web
```

默认 H5 监听 4310，API 的宿主机端口由 KNOWLEDGE_API_PORT 控制。更改端口后按实际值检查。生产与新环境部署始终使用 deploy/docker-compose.yml 并显式设置 OPS_NODE_NAME；根目录 compose.yaml 是本地开发入口。自定义 EPISODE_PLAN_DIR 时建议填写宿主机绝对路径。

| 按需安装 | 入口 |
|---|---|
| 公开 Nginx 与广州服务 | [网关安装与验收](gateway/README.md) · [广州部署顺序](guangzhou/README.md) |
| DNS 与跨节点 Mesh | [Cloudflare 部署架构](cloudflare/README.md) |
| K3s PostgreSQL 备份及恢复校验 | [备份部署](pg-backup/README.md) |
| 广州图片服务 | [systemd 单元](image-service/folio-image.service) |
| iOS 沙盒账户与验单服务 | [独立 Compose](compose.ios-sandbox.yaml) |

分集登记、取证、图片导入和定稿投递随 [sync 生产流程](../apps/sync/ops/production/README.md) 维护。部署新环境后可单独启用这些业务任务。
