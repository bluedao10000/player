# 广州图片服务 · 火影忍者灰度

本期交付 2002 年 TV 版已发布的 5 张场景图、6 张角色肖像。权威范围见 私有暂存目录中的 manifest.txt。图片原件按 SHA-256 校验，公开响应保留已有字节、格式和 URL。私有预览单独用 libvips 预生成 780 × 1040 WebP。读取请求由 [Rust / Axum 服务](../../../image/src/main.rs) 处理。

## 运行路径

- Cloudflare 命中缓存时直接返回原件。未命中时，阿里云的 [Nginx 网关](../../../../deploy/cloudflare/player-ingress.conf) 仅将清单内的精确路径转到本机 14581，经 Unix socket 和 Mesh 到广州 GZ_MESH_IP:14580。广州监听器限制源为 ALI_MESH_IP，再经受限 socket 转到回环地址上的 Axum。[两端中继](../../../../deploy/cloudflare/mesh/images.yaml)
- 公开图保留一年 immutable 缓存和 ETag。灰度源响应带 `X-Folio-Image: gz-naruto-v1`。网关遇到 502、503、504 时沿原 H5 读图链返回旧文件。旧 CDN 对象到期前可能没有灰度响应头。
- 私有预览路径为 `/image-preview/art/…`。签名绑定完整路径及到期秒数，凭据放在广州 `/var/lib/folio-image/preview.key`，授权运维端使用 [签发工具](sign-preview.py)。无签名、过期或篡改返回 403，成功和拒绝都返回 private, no-store。预览仅供内部签发，公开工作台与读者客户端均无需持有密钥。

## 部署及回退

1. 设置 READER_PUBLIC_URL 与 PILOT_WORK_SLUG，用 `python3 apps/sync/ops/image-import/import.py STAGING_DIR` 拉取作品、章节与图片，核对其他作品世界无相同引用，复核生成的 manifest.txt 和 player-ingress.conf，再将配置中的示例域名替换为本站域名。把暂存目录送到广州，在该目录运行 [prepare.sh](prepare.sh)。缺图、摘要不匹配或裁切失败时停止。
2. 在广州使用与目标系统匹配的 libvips 运行库和 Rust 1.95 构建 `cargo build --release --locked`，安装二进制到 /usr/local/bin/folio-image。安装 manifest.txt、library/ 和随机生成的 32 字节签名密钥到 /var/lib/folio-image，限制密钥为 root 与服务用户可读。在回环地址启动 [服务](../../../../deploy/image-service/folio-image.service)、[socket](../../../../deploy/image-service/folio-image-proxy.socket) 与[转接器](../../../../deploy/image-service/folio-image-proxy.service)。目前广州 socket 的属组对应 GID 1001。
3. 使用 Mesh 配置工具的 `image ALI_MESH_IP GZ_MESH_IP` 路径放行限定流量。在现有 Mesh Compose 上叠加 images.yaml，广州启动 image-listener；阿里云启动 image-client 和 image-loopback，后两者共用仅 UID 1000 可访问的 /var/lib/folio-image-mesh。先从阿里云 127.0.0.1:14581 取图核对响应头及摘要。
4. 部署网关前保存当前配置，核对精确路径与清单，执行 nginx -t 后 reload。回退时移除本期两组精确路径及其命名回退 location，再测试配置并 reload。保留原 H5 文件，公开对象即使已缓存仍然与原文件一致。紧急撤销预览时关闭该路由或轮换签名密钥。

广州灰度仍通过受控导入写入。独立图片卷和管理 API 可在影子实例验收，部署参见[图片服务手册](../../../image/README.md)。
