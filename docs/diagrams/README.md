# 更新架构图

编辑对应视图的 Mermaid 文件，技术图位于上一层的 [architecture.mmd](../architecture.mmd)。每个节点用一行 `%% 节点ID | 职责或验收含义 | 仓库相对源码路径` 记录证据。[render.cjs](render.cjs) 核对节点和文件，然后从同一输入导出四张 SVG、两张历史链接使用的 PNG，以及嵌入 SVG 的离线图集。

需要重新生成时，可在仓库根目录使用临时工具目录。已安装 Google Chrome 的环境使用下面的命令。运行应用、阅读文档均无需安装这些工具。

```sh
diagram_tools=$(mktemp -d)
npm install --prefix "$diagram_tools" mermaid@11.17.2 playwright@1.62.1
NODE_PATH="$diagram_tools/node_modules" CHROME_CHANNEL=chrome node docs/diagrams/render.cjs
```

打开[图集](../architecture.html)，在桌面和窄屏切换四个视图，点击节点，核对说明、源码和图源链接。图集模板为 [atlas.template.html](atlas.template.html)。

单独更新[待开发功能 PNG](backlog.png) 时，编辑 [backlog.mmd](backlog.mmd)，在上述渲染命令末尾加上 `backlog`，即可一起导出对应的 SVG 和 PNG。
