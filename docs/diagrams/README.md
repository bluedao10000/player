# 更新架构图

编辑对应视图的 Mermaid 文件，技术图位于上一层的 [architecture.mmd](../architecture.mmd)。每个节点用一行 `%% 节点ID | 职责或验收含义 | 仓库相对源码路径` 记录证据。[render.cjs](render.cjs) 核对节点和文件，然后从同一输入导出各视图的 SVG、产品／业务／功能地图 PNG，以及嵌入 SVG 的离线图集。

需要重新生成时，可在仓库根目录使用临时工具目录。已安装 Google Chrome 的环境使用下面的命令。运行应用、阅读文档均无需安装这些工具。

```sh
diagram_tools=$(mktemp -d)
npm install --prefix "$diagram_tools" mermaid@11.17.2 playwright@1.62.1
NODE_PATH="$diagram_tools/node_modules" CHROME_CHANNEL=chrome node docs/diagrams/render.cjs
```

打开[图集](../architecture.html)，在桌面和窄屏切换各视图，点击节点，核对说明、源码和图源链接。图集模板为 [atlas.template.html](atlas.template.html)。

单独更新[待开发功能 PNG](backlog.png) 时，编辑 [backlog.mmd](backlog.mmd)，在上述渲染命令末尾加上 `backlog`，即可一起导出对应的 SVG 和 PNG。

[功能地图](features.png) 按领域列出开发状态；更新时同步维护[图源](features.mmd)与[功能明细](../功能地图.md)。

[本地工作台功能地图](workbench.svg) · [PNG](workbench.png) · [Mermaid 源文件](workbench.mmd)

[2026-11 交付图](delivery-2026-11.svg) · [PNG](delivery-2026-11.png) · [Mermaid 源文件](delivery-2026-11.mmd)：运营中心 Phase 3、Knowledge P0 与 P1、性能和快照修复，覆盖 `91e92da` 至 `b7ebc8b`，并列出未完成项。单独更新时在渲染命令末尾加上 `delivery`。
