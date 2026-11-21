# Folio 时光 · 作品世界

从作品找到人物，从人物读到出场章节，再从一幕场景回到故事里的那一段。H5 提供公开阅读、作品搜索和私人分支试玩；数据服务把上游作品档案、分集取证和已验收插画组织成可发布的知识库。

**开发入口**　[AGENTS.md](AGENTS.md) 按任务指向架构、源码和验证方法。[文档索引](docs/README.md) · [离线架构图集](docs/architecture.html)

部署新环境请使用 [部署入口](deploy/README.md)，单集制作与图片导入见 [sync 生产任务](apps/sync/ops/production/README.md)。

## 本地启动

需要 Node 24.13+（24.x）、npm、Go 1.25+ 和 Docker Compose。首次创建 `.env` 后设置数据库密码。已有环境沿用原文件。新库会在后台导入上游，样板的公开范围取决于本地数据和模型任务进度。

```sh
test -f .env || cp .env.example .env
# 首次启动前编辑 .env，填写 POSTGRES_PASSWORD
npm ci
docker compose up -d --build
npm run dev:web
```

打开 [H5 首页](http://127.0.0.1:4310/)、[知识库](http://127.0.0.1:4310/knowledge) 或 [数据工作台](http://127.0.0.1:4310/workbench)。模型配置、端口、空库启动及恢复步骤见[运维手册](docs/运维手册.md)。
