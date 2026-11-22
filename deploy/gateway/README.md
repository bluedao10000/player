# 杭州公网网关

网关只持有路由配置、公开图片路径清单和短期日志。Nginx 使用 Linux host 网络访问既有回环灰度中继，只监听 Unix socket。公网请求从专用 cloudflared 进入；到广州的新连接通过三个受限 Unix socket 进入现有 Mesh 网络命名空间。[路由模板](nginx/routes.conf) · [广州服务](../guangzhou/README.md) · [分阶段切换与网络矩阵](../../docs/部署架构.md)

## 建立旁路入口

1. 准备 Linux、Docker Compose v2.24.4+、Python 3、curl、flock 和 /usr/local/bin/cloudflared。核对容器 UID 1000 在宿主机为专用可信身份。安装脚本使用 GNU coreutils、systemd 和 /usr/bin/docker。只有连接器与受信运维账号可以进入 /run/player-edge。现有 Tunnel、Nginx、转发器及 DNS 继续服务。
2. 从本目录以 root 运行 `scripts/install.sh`。脚本安装独立单元和 /opt/player-edge、/etc/player-edge 下的文件。更新版本时从新检出重新安装，保留 settings 和历史渲染配置。每个 release 同时快照 Compose 和镜像摘要，保留已验收镜像；Tunnel 与 Mesh 单元另存旧版本。
3. 用专用测试 Tunnel 的 UUID、核验过的指标回环端口和测试域名，分别替换 /etc/player-edge/tunnel.yml.in 中的 TUNNEL_UUID、METRICS_PORT、PUBLIC_HOST，保存为 /etc/player-edge/tunnel.yml。它是**本地管理的 Tunnel**。将该隧道的 JSON 凭据以 root:root 0600 安装到 /etc/player-edge/tunnel.json，确认 catch-all 返回 404。测试与正式 hostname 需要各自明确授权的 Cloudflare 路由。旧远程管理令牌继续用于旧单元。[Tunnel 模板](cloudflare/tunnel.yml.in)
4. 将已核验的 image-import 清单安装到 settings 的 MANIFEST 路径。填写 PUBLIC_HOST、旧账户站 ACCOUNT_HOST、各组上游。确认全部 11 个旧图路径与原入口相同。模板允许每组单独选 Mesh socket 或回环 IP 加端口，未填、空清单、含通配符或非法文件名会中止部署。测试域名的页面加 noindex。正式配置使用 www.codeiy.com。开发、预发和正式配置分别留档。[清单来源](../../apps/sync/ops/image-import/README.md)
5. 按 [Mesh 手册](mesh/README.md) 接好广州三个服务。初期 WEB、STATIC、AUTH、BILLING 和 FALLBACK 可以分别指向经核实的**原 H5**回环端口；API 组必须指向 Gin。旧网关端口不能用作兜底上游，否则可能循环回源。新 /i/ 与旧灰度分别设置。检查工作台只在私有环境可用。
6. 在杭州运行以下命令。首次部署脚本在隔离的运行目录做 nginx -t，然后启动 Nginx。同镜像更新先切换已渲染配置的符号链接再发 HUP，旧 worker 排空已有连接；镜像或容器配置变化时 Compose 重建网关，须安排短暂重连窗口。脚本用锁串行更新，等待本地检查 socket 返回新版本号，失败时恢复上版。版本检查仅证明配置加载，继续做下方端到端探测后再切流。

```sh
cd /opt/player-edge
scripts/deploy.sh
scripts/healthcheck.sh
cloudflared tunnel --config /etc/player-edge/tunnel.yml ingress validate
systemctl enable --now player-edge-tunnel
curl -fsS "http://127.0.0.1:${METRICS_PORT}/ready"
```

## 路由与缓存

| 请求 | 行为 |
|---|---|
| /、/work/、/story/、/art/ 其余路径、/ios-data/、TanStack server function | 广州 Web；静态 /assets/ 可独立切换，页面与构建资源需来自兼容版本 |
| /api/v1/knowledge/discovery、worlds 及已登记详情 | 仅 GET/HEAD 走 Gin。其余 /api/ 默认 404，增加公开接口时同时更新两端白名单 |
| /reader-auth/、/ios-account/ | 广州 H5 校验同源、转换 Cookie/Bearer、转发 Gin 的 /api/v1/reader/。微信回调同时保留 folio_flow 和 folio_reader；原生接口保留 Authorization、X-Reader-ID |
| /api/auth/callback/wechat、/api/auth/callback/github、/billing/alipay-return | 仅 GET 重定向旧账户站，保留 query；/guides/ 保留原重定向。/api/wechat-mini/ 使用验证证书和域名的 HTTPS 原方法代理 |
| POST /api/billing/alipay/notify | 原表单送 H5。仅当 Host 为 www.codeiy.com 且唯一 out_trade_no 为 32 位小写十六进制，H5 才转旧账户站。其余由 Gin 验签 |
| /i/、精确清单 /art/、精确清单 /image-preview/art/ | 只读。清单原图仅在 502/503/504 时回退一次旧图；404、签名失败和新资产均不回退。签名预览强制 private, no-store |
| /workbench*、/editor-api*、/editor-preview*、/v1/、后台 API | 404；广州 Web 强制 PLAYER_PUBLIC_ONLY，Gin 禁用公开工作台凭据 |

Cloudflare 对带 sig 或 exp 的请求、预览、认证、iOS 账户、支付、携带授权或 Set-Cookie 的响应绕过缓存。/i/ 仅在源响应明确 public 时缓存，缓存键包含完整变换查询串。公开版本原图保留一年 immutable，变体服从源 TTL。禁止对 HTML、错误或全站使用 Cache Everything。灰度对比同时记录 CF-Cache-Status、ETag、Range 和源响应头，区分旧缓存命中。

仅接受 socket 对端转来的 CF-Connecting-IP。Cloudflare 边缘必须覆盖此头，仅授权本隧道回源，禁用未经审查的改写 Worker。Nginx 从白名单重建上游请求头，单值 X-Forwarded-For、Host 和 https 由网关写入。访问日志含路径、上游和耗时，限制错误日志的权限及保留期，Nginx 上游错误可能含查询串。默认按连接 IP 使用独立的登录、通知和读取令牌桶，429 计数与运营方回调峰值一起复核。网关关闭通用重试和请求/响应缓冲；SSE 心跳与 WebSocket ping 需短于 90 秒的上游空闲超时。

## 验证、定位故障与回退

```sh
cd /opt/player-edge
docker compose ps
docker compose logs --since 10m nginx
journalctl -u player-edge-tunnel -u player-edge-mesh --since '10 minutes ago'
scripts/healthcheck.sh --upstreams "/i/$PUBLISHED_ASSET_UUID"
curl --unix-socket /run/player-edge/origin.sock -fsS -H "Host: $PUBLIC_HOST" http://local/healthz
curl --unix-socket /var/lib/player-edge/mesh/api.sock -fsS http://local/api/v1/knowledge/discovery
curl --unix-socket /run/player-edge/origin.sock -s -o /dev/null -w '%{http_code}\n' -H "Host: $PUBLIC_HOST" http://local/workbench
```

依次检查 Tunnel ready、本机版本、三个 socket 的属主与监听、Mesh enrolled 状态、/32 策略、广州 Mesh IP 绑定、Docker DNS 和服务自身健康。Tunnel 正常而正文 502 时直接探测 web.sock；API 503 再查 Gin /healthz 与 PostgreSQL。以批准的公开作品检查 HTML 正文、canonical、锚点和哈希静态资源。用测试账号完成 OAuth、iOS challenge、验单和重复测试通知。测试域名不能触发旧单 www 判断；用隔离账户端及原 Host 演练，再在授权窗口观察正式回执。模拟断开测试图片上游，核对旧图字节；预览无签名、篡改、过期应被拒绝并禁缓存。

配置回退运行 `scripts/rollback.sh release-已验收版本`，它用该版本的 Compose 和固定摘要再次校验配置，恢复容器或热加载。首次旁路失败时保留原 Tunnel 入口，停用新 Tunnel。若已切域名，恢复原先保存的 Cloudflare route/origin 配置，核对公网查询与所有回调的唯一写入端。已发生写入时按阶段 4 反向追平后再切写。仓库中的 [隔离回归](tests/smoke.sh) 可在带 Docker、Python 3 的开发机运行，不接入任何隧道。
