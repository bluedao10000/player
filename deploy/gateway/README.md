# Folio 公网 Nginx

`www.codeiy.com` 提供 Vite 构建的 SPA，`img.codeiy.com` 提供广州图片库。两个域名使用同一份 Nginx 配置和同一份正式构建。Gin 读 API 和读者业务接口走私网。图片管理只在受限网络使用。旧工作台和编辑路径返回 404。

## 部署前准备

1. 将两个域名指向经核实的 Nginx 入口，准备覆盖两域名的公共 CA 完整证书链及私钥。普通 Docker 入口监听 80/443。按防火墙和证书签发流程放行所需端口。
2. 复制 [.env.example](.env.example) 到权限受限的 settings。填写 `API_UPSTREAM`、`READER_API_UPSTREAM` 和 `IMAGE_UPSTREAM`。端点可使用私网 TCP 或 Nginx 可访问的 Unix socket。填写图片上游的 `IMAGE_UPSTREAM_HOST`。上游若使用 HTTPS，设置 `IMAGE_UPSTREAM_SCHEME=https`，Host 同时用于 SNI 及证书校验，私有 CA 用 `UPSTREAM_CA_BUNDLE` 指定容器内可读路径。HTTP 上游应经受限加密 Mesh 或同机私网。
3. 启动 [广州私网服务](../guangzhou/README.md)。如采用现有 Mesh，使用 [中继配置](mesh/README.md)。从网关验证发现 API、读者业务服务和图片库就绪。管理 API 的 token 只交给广州资产调用者。
4. 在维护窗口暂停正式发布。从读者数据库导出已发布的图片引用，并准备对应 JPEG、WebP 原件。归档根目录和读者库 DSN 均由运维按实际挂载配置。图片原件导入和核验完成后再开放两个域名，并恢复单集投递。

```sh
psql "$READER_INVENTORY_DSN" -X -At -v ON_ERROR_STOP=1 \
  -f deploy/gateway/scripts/published-images.sql > published-images.txt
# 在能访问广州管理端口的私有环境中设置 IMAGE_ADMIN_URL 和 IMAGE_API_TOKEN。
python3 deploy/gateway/scripts/seed-images.py published-images.txt \
  --scene-archive "$SCENE_ARCHIVE" --portrait-archive "$PORTRAIT_ARCHIVE"
# 使用相同参数加 --verify-only 可以复查 SHA-256。
```

脚本同时导入仓库的原创插画和品牌位图，校验每个有摘要的归档文件，缺图时中止。JPEG 文件名为 SHA-256，WebP 文件名为 SHA-256 加 `.webp`。原件不联网下载。新增原创位图后运行 `node apps/web/generate-images.mjs` 并上传该版本的图片。

## Docker

```sh
deploy/gateway/scripts/deploy.sh docker /etc/folio-edge/settings
```

`EDGE_CONFIG_DIR`、`EDGE_TLS_DIR`、`EDGE_RELAY_DIR` 必须是宿主机绝对路径。证书目录提供 `fullchain.pem`、`privkey.pem`。纯 TCP 场景也需创建一个空的 relay 目录。Nginx worker UID 1000，需要目录遍历权限和连接 socket 的权限。部署脚本构建镜像、验证 Compose 与 Nginx 配置，再替换唯一 Nginx 容器。CI 产出 `folio-static-site` 并运行隔离验证。正式部署在已配置入口执行。

## 独立 Nginx

使用带 HTTP/2 模块的 Nginx 1.25.1 或以上版本，并准备 Node 24、Python 3、rsync。设置 `NGINX_WORKER_USER`、`TLS_FULLCHAIN`、`TLS_PRIVATE_KEY`、`EDGE_CONFIG_DIR`。确保 worker 可写 `IMAGE_CACHE_DIR` 且可读 `SITE_ROOT`。以能管理 Nginx 的账号运行下令，所用服务单元应以同一 `nginx.conf` 启动。

```sh
deploy/gateway/scripts/deploy.sh native /etc/folio-edge/settings
```

也可从 CI 解包 `folio-static-site` 到 `SITE_ROOT`，执行渲染、`nginx -t -c "$EDGE_CONFIG_DIR/nginx.conf"` 和同配置加载。证书轮换同样触发 Nginx reload。

## 复用本机 Cloudflare Tunnel

杭州生产入口可由现有 Tunnel 接收两个公开域名，Nginx 仅监听宿主机回环。渲染时设置 `CLOUDFLARE_LOCAL_TUNNEL=1`，并填写实际 `HTTP_LISTEN`、`HTTPS_LISTEN` 回环端口。容器使用宿主机网络并明确配置可访问 Mesh socket 的 worker UID/GID。Cloudflared 的每个 ingress 使用 HTTPS 本机端口，设置对应的 `originServerName` 和 `httpHostHeader`，通过 `caPool` 校验本机证书。Nginx 读取 Tunnel 提供的客户端地址，把公网 HTTP 请求重定向到同域 HTTPS。

`www` 与 `img` 的代理 DNS 记录指向同一 Tunnel。切换前先从公网校验图片证书、原件摘要和变体，再切换站点入口。切换时暂停发布定时器，确认在途任务完成并更新发送端和接收端，验证公开读链路后恢复。

## 路由、缓存与验收

| 入口 | 行为 |
| --- | --- |
| `www /work/*`、`/story/*` | SPA 入口，HTML 须重新验证。浏览器经同源白名单读取 Gin |
| `www /assets/*` | 仅返回存在的文件，带构建 hash 的文件缓存一年，Gzip 压缩文本资源 |
| `www /reader-auth/*`、`/ios-account/*`、`/ios-data/*`、`/api/billing/alipay/notify` | 私网 Reader API 保留 Cookie、App 凭证和支付原始表单。iOS 上线须使用支持独立图片域名的客户端版本 |
| `img /i/*` | 只读代理。转发 Range 和验证器，保持源 Content-Type。公开原件缓存一年，变体一天。共享缓存只存明确公开的完整响应。签名、携带凭据及 Range 请求绕过共享缓存 |
| `/healthz`、`/readyz` | 检查 Nginx 与各站就绪。站点就绪检查入口 HTML，图片就绪请求广州 |
| 两站 HTTPS | 未知主机拒绝，HTTP 升级到 HTTPS。图片 CORS 允许配置的站点 Origin，公开图片可跨域嵌入 |

若接入 CDN，保留全部查询参数作为缓存键，遵守源 Cache-Control，对签名、Cookie、Authorization、Range 请求绕过共享缓存。CDN 与源站采用校验源证书的 HTTPS。启用上游真实客户端地址时，只信任已核对的 CDN 代理网段。网关磁盘缓存和广州衍生缓存各有独立容量。

```sh
deploy/gateway/tests/smoke.sh
curl -fsS https://www.codeiy.com/readyz
curl -fsS https://img.codeiy.com/readyz
curl -sSI "https://img.codeiy.com/i/$PUBLIC_IMAGE_SHA?format=webp&w=600"
```

公网验收应在移动端直达章节和场景并核对段落回跳。用真实样本核对响应摘要、图片缓存命中、登录与支付回调，并确认两个域名对外提供完整证书链。访问日志记录规范化路径、上游状态、耗时与缓存状态。

使用 Playwright 运行 390px 浏览器用例时，设置 `EDGE_SMOKE_BROWSER=1` 及可解析 Playwright 模块的 `NODE_PATH`。浏览器用例使用隔离监听器，并记录独立图片域名的实际资源请求。
