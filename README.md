# 二次元剧情内容站

产品设计、领域模型、SEO/GEO 和八阶段验收见 [产品架构](docs/产品架构.md)。[交互架构图集](docs/architecture.html) 标注已实现与待开发的能力，可离线查看并点击节点核对源码。可编辑的 Mermaid 图源包括[产品架构](docs/diagrams/product.mmd)、[业务流程](docs/diagrams/business.mmd)、[技术架构](docs/architecture.mmd)和[双云部署](docs/diagrams/deployment.mmd)。

目前已搭建阶段 1 的服务骨架，并在 apps/web 接入可点击的「潮汐邮局」H5 产品设计原型。apps/api 运行 Gin 并连接 PostgreSQL 作品知识库，apps/agent 打开 Pi Durable 的 SQLite 存储并启动恢复调度。packages/shared 提供健康响应和跨语言 JSON Schema，packages/seo 为后续发布页面预留。阶段 1 无需模型密钥。

[网页数据工作台](http://127.0.0.1:4310/workbench) 展示数据服务的自动同步、质检记录和图片优化规划拓扑。

各项已实现能力的实测范围见[架构实现核验](docs/架构实现核验.md)。

## 体验产品原型

打开首页，从“港口人物”点阿澪进入角色主页，选择《末班渡船》阅读。在阅读页可加入书架，或从旧渡口进入剧情，选择两条不同的预设分支。返回书架可重读收藏；重新打开同一场景可看到上次选择。收藏与选择保存在当前浏览器。公开页面使用原创 fixtures 和本地插画，剧情使用预设文本。

## 启动与验收

在项目根目录运行，使用 Node 24.13+（24.x）、npm、Go 1.25+ 和 Docker Compose。首次启动先复制 .env.example 为 .env，填写随机 POSTGRES_PASSWORD。

```sh
docker compose up -d --build
npm ci
npm run check
npm run dev
```

作品库与定时 worker 独立于前端运行。可直接[搜索作品](http://127.0.0.1:4310/knowledge)，或查看[知识库架构、配置和运维说明](docs/knowledge-base.md)。

另开终端执行以下请求。三个健康接口应分别返回 HTTP 200、对应 service 和 status 为 ok。首页的原始 HTML 应含有 h1，无需执行 JavaScript。

```sh
curl -fsS http://127.0.0.1:4310/healthz
curl -fsS http://127.0.0.1:4311/healthz
curl -fsS http://127.0.0.1:4312/healthz
curl -fsS http://127.0.0.1:4310/ | rg -o '<h1>.*?</h1>' 
```

也可单独执行 npm run dev:web、npm run dev:api、npm run dev:agent。Compose Gin 默认在 4321 端口，本地开发 Gin 在 4311，使用同一 PostgreSQL。知识数据保存在 Docker 命名卷，Agent 的执行检查点默认保存在 data/agent.sqlite。
