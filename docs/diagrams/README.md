# 架构与采集图

先按问题选图，再按需看图源。四张 HTML 图可离线打开；Mermaid 只保留 `.mmd` 源，渲染产物不再提交。

| 问题 | 图 | 依据 |
| --- | --- | --- |
| 一集现在怎样走到读者？ | [当前分集生产链](current.html) | sync、Go 知识库服务与 Agent 的现有入口 |
| 采集要改成什么样？ | [目标采集架构](target-collection.html) | [广州定时采集评审](../架构评审-广州定时采集.md) §4（待建设） |
| 哪些路径能跨信任边界？ | [部署与信任边界](deployment-boundaries.html) | [部署架构](../部署架构.md)、`deploy/guangzhou`、`deploy/gateway`（目标拓扑，现网未核对） |
| 单一官方来源为什么能通过？ | [分集来源证据策略](source-evidence.html) | `source_evidence.go` 与四个 Go、一个 TS 入口 |

## Mermaid 图源

改图后按需重绘，不再保留 SVG／PNG。

| 图源 | 视图 | 状态 |
| --- | --- | --- |
| [../architecture.mmd](../architecture.mmd) | 当前技术架构 | 现状 |
| [deployment.mmd](deployment.mmd) | 当前网络与部署 | 现状 |
| [target-deployment.mmd](target-deployment.mmd) | 目标部署（合并到广州） | 目标 |
| [media-platform.mmd](media-platform.mmd) | 目标图片架构 | 目标 |
| [business.mmd](business.mmd) | 知识库构建流程 | 现状 |
| [product.mmd](product.mmd) | 产品架构 | 现状 |
| [features.mmd](features.mmd) | 功能地图 | 基线 |
| [backlog.mmd](backlog.mmd) | 待开发功能 | 计划 |
| [workbench.mmd](workbench.mmd) | 旧工作台 | 历史 |
| [delivery-2026-11.mmd](delivery-2026-11.mmd) | 2026-11 交付 | 历史 |

每个节点用一行 `%% 节点ID | 职责或验收含义 | 仓库相对源码路径` 记录证据；[render.cjs](render.cjs) 会核对节点与文件，缺一行就报错。

## 重绘 Mermaid

在仓库根目录使用临时工具目录。运行应用和阅读文档都不需要这些依赖。

```sh
diagram_tools=$(mktemp -d)
npm install --prefix "$diagram_tools" mermaid@11.17.2 playwright@1.62.1
NODE_PATH="$diagram_tools/node_modules" CHROME_CHANNEL=chrome node docs/diagrams/render.cjs
```

输出为各视图的 SVG、产品／业务／功能地图 PNG，以及嵌入 SVG 的离线图集 `docs/architecture.html`，都写在本地。它们已加入 `.gitignore`；需要时生成，只挑要发布的文件带走。

只重绘单个视图时，在命令末尾加 `backlog`、`delivery` 或 `workbench`。

## 校验 HTML 图

HTML 图是手写 SVG。改完用 diagram-design 技能的 `scripts/self_check.py` 与 `scripts/verify_geometry.py` 检查，再用无头 Chrome 截图看真实排版。重点：文字不越框、标签遮罩不压节点、箭头不穿卡片、窄屏可横向滚动。
