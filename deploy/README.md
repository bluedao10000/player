# 新环境部署

从仓库根目录运行以下命令。基础 Compose 启动 PostgreSQL、Gin API、sync 和 Pi Durable，H5 由 Node 运行。首次部署需要 Docker Compose v2.20 以上及 Node 24.13+。数据库、媒体归档和 Agent 检查点使用命名卷。

在 `.env` 中为 `OPS_NODE_NAME` 设置该部署唯一的节点名（例如 `gz`、`sv` 或本地开发用 `local`），`sync` 启动时会拒绝缺少该值的长期运行配置。`APP_VERSION` 用于 Agent 版本心跳，可填写发布标签或 commit。分节点 systemd 部署在 `lane.env` 中设置 `OPS_NODE_NAME` 和 `APP_VERSION`，见[生产调度说明](../apps/sync/ops/production/README.md)。

作品世界目录导入或分集登记新建作品时，会在同一事务内自动补齐已有 `mio`、`bebop%` 图片候选的批次归属；重复导入不会重复创建映射。

```sh
test -f .env || (umask 077 && cp deploy/.env-example .env)
# 在 .env 中设置随机 POSTGRES_PASSWORD，按需填写模型与登录服务的配置
docker compose --env-file .env -f deploy/docker-compose.yml config --quiet
docker compose --env-file .env -f deploy/docker-compose.yml up -d --build
curl -fsS http://127.0.0.1:4321/healthz
npm ci
npm run dev:web
```

默认 H5 监听 4310，API 的宿主机端口由 KNOWLEDGE_API_PORT 控制。更改端口后按实际值检查。根目录 compose.yaml 引用同一份部署定义，原有 docker compose 命令继续可用。自定义 EPISODE_PLAN_DIR 时建议填写宿主机绝对路径。

| 按需安装 | 入口 |
|---|---|
| 新杭州独立网关、广州业务栈（分期切换） | [网关安装与回滚](gateway/README.md) · [广州部署顺序](guangzhou/README.md) |
| 既有公网入口、Tunnel、跨节点 Mesh | [Cloudflare 部署架构](cloudflare/README.md) |
| K3s PostgreSQL 备份及恢复校验 | [备份部署](pg-backup/README.md) |
| 图片服务及本机 socket 转发 | [systemd 单元](image-service/folio-image.service) |
| iOS 沙盒账户与验单服务 | [独立 Compose](compose.ios-sandbox.yaml) |

分集登记、取证、图片导入和定稿投递随 [sync 生产流程](../apps/sync/ops/production/README.md) 维护。部署新环境后可单独启用这些业务任务。
