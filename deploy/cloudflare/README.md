# Cloudflare 私网与 DNS

公网域名和 Nginx 的部署参数按[网关手册](../gateway/README.md)配置。已有 Cloudflare Mesh 继续为制作、备份、发布与网关私网连接提供节点身份和加密传输。

每台已注册节点沿用自己的注册状态卷与环境文件。[基础 Compose](mesh/compose.yaml) 启动连接器，业务按需要叠加[发布和备份中继](mesh/operations.yaml)或[公网网关私网中继](../gateway/mesh/README.md)。Gateway 策略只放行已登记源、目标和端口。[发布与备份策略脚本](mesh/configure-peer.py)

```sh
docker compose -f compose.yaml -f operations.yaml --profile "$ROLE" config --quiet
docker compose -f compose.yaml -f operations.yaml --profile "$ROLE" up -d
```

数据库备份用受限账户做只读校验。定稿投递按[发布手册](../../apps/sync/ops/reader-release/README.md)核对唯一回执、章节、段落锚点和图片摘要。
