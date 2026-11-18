# 二次元剧情内容站

产品设计、领域模型、SEO/GEO 和八阶段验收见 [产品架构](docs/产品架构.md)。[交互架构图](docs/architecture.html) 可直接在浏览器打开，[Mermaid 源图](docs/architecture.mmd) 可编辑。

目前已搭建阶段 1 的服务骨架，并在 apps/web 接入可点击的「潮汐邮局」H5 产品设计原型。apps/api 运行 Gin 并连接业务 SQLite，apps/agent 打开 Pi Durable 的 SQLite 存储并启动恢复调度。packages/shared 提供健康响应和跨语言 JSON Schema，packages/seo 为后续发布页面预留。阶段 1 无需模型密钥。

## 体验产品原型

打开首页，从“港口人物”点阿澪进入角色主页，选择《末班渡船》阅读。在阅读页可加入书架，或从旧渡口进入剧情，选择两条不同的预设分支。返回书架可重读收藏；重新打开同一场景可看到上次选择。收藏与选择保存在当前浏览器。公开页面使用原创 fixtures 和本地插画，剧情使用预设文本。

## 启动与验收

在项目根目录运行。使用 Node 24.13+（24.x）、npm、Go 1.25+ 和 C 编译器（Go SQLite 驱动需要 CGO）。

```sh
npm ci
(cd apps/api && go mod download)
npm run check
npm run dev
```

另开终端执行以下请求。三个健康接口应分别返回 HTTP 200、对应 service 和 status 为 ok。首页的原始 HTML 应含有 h1，无需执行 JavaScript。

```sh
curl -fsS http://127.0.0.1:4310/healthz
curl -fsS http://127.0.0.1:4311/healthz
curl -fsS http://127.0.0.1:4312/healthz
curl -fsS http://127.0.0.1:4310/ | rg -o '<h1>.*?</h1>' 
```

也可在不同终端分别执行 npm run dev:web、npm run dev:api、npm run dev:agent。停止服务用 Ctrl-C。业务库和运行库默认保存在 data/app.sqlite、data/agent.sqlite。一个 Agent 运行库只运行一个写进程。

默认参数可直接运行。自定义端口或数据库路径时复制 .env.example 为 .env，修改后在同一个终端执行 `set -a; . ./.env; set +a` 再启动。Web 仅读取 WEB_PORT，模型接入在阶段 5 实施。
