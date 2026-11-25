# Folio 图片服务

Axum + Tokio 提供流式上传与分发，SQLite WAL 保存资产、目录、发布引用和 AI 任务，libvips 负责解码与转换。原图按内容寻址存入本地卷，资产 UUID 与存储键独立。旧广州清单服务和新资产目录可并行运行。

[OpenAPI](openapi.yaml) · [架构与数据归属](../../docs/图片库架构.md) · [压测报告](PERFORMANCE.md)

## 部署

从 `apps/image` 执行。准备两份独立随机值填入 `.env` 的 `IMAGE_API_TOKEN` 与 `IMAGE_SIGNING_KEY_HEX`。每份可用 `openssl rand -hex 32` 生成。也可挂载密钥文件并使用 `.env.example` 所列的 `*_FILE` 设置。签名密钥文件按原始字节读取，十六进制文本用 `IMAGE_SIGNING_KEY_HEX`。

```sh
cp .env.example .env
chmod 600 .env
docker compose up -d --build
curl -fsS http://127.0.0.1:14680/readyz
docker compose logs image
```

Compose 在宿主机回环发布 14680，容器内监听 14580。实例在独立的 `folio-image_image-data` 卷内持久化，容器 UID/GID 为 10001。单卷运行一个写入实例，服务启动时获取独占锁。若使用独立 SSD/NVMe 绑定挂载，提前让该 UID 可写并使用同一文件系统承载 `objects`、`scratch` 和 `derivatives`。通过 `RUST_LOG` 设置日志级别。原生构建使用 Rust 1.95、libvips 8.16 及具备 AV1 编码和解码的 libheif，默认监听 127.0.0.1:14580。


## REST API

| 接口 | 用途 |
| --- | --- |
| POST `/v1/assets/upload?name=scene.jpg` | 原始二进制上传，可设 `directory_id`、`source` 和 `previous_id` |
| GET `/v1/assets` | 游标分页，可按目录、前序版本、AI 来源资产或 `q` 过滤 |
| GET / PATCH / DELETE `/v1/assets/{id}` | 详情、改名或替换元数据、逻辑删除 |
| GET `/v1/assets/{id}/download` | 管理端下载原图 |
| POST `/v1/assets/{id}/move` | 修改逻辑目录，传 `directory_id: null` 移至根目录 |
| POST `/v1/assets/{id}/publish` | 写入 `reference` 并固定公开版本 |
| GET `/v1/assets/{id}/sign` | 为原图或指定变换签发 600 秒 URL |
| POST `/v1/assets/{id}/prewarm` | 预热某套变换参数 |
| GET / POST `/v1/directories` | 目录分页和创建，可指定父目录 |
| DELETE `/v1/directories/{id}` | 删除空目录 |
| GET / HEAD `/i/{id}` | 分发原图或变体 |
| POST `/v1/ai/{analyze,generate,edit}` | 受理异步 AI 任务 |
| GET `/v1/jobs/{id}`、`/v1/stats` | 查询任务和缓存计数 |
| GET `/healthz`、`/readyz`、`/openapi.yaml` | 健康、就绪和接口文档 |

`/v1/` 使用 `Authorization: Bearer <token>`，管理端和签发端应只走可信网络。客户端上传的扩展名不参与识别。支持 JPEG、PNG、WebP、AVIF，可省略 Content-Type 或用 `application/octet-stream`。SVG 禁止上传。默认上传 25 MiB、源图 3200 万像素且单边不超过 16384。被发布、被引用或被排队任务使用的资产无法删除。删除私有资产会隐藏目录记录和读链路，原始对象继续保留。上传新版本产生新 UUID，发布者应在将该 UUID 写入外部业务前先调用 publish。

```sh
base=http://127.0.0.1:14680
curl -fsS -H "Authorization: Bearer $IMAGE_API_TOKEN" \
  -H 'Content-Type: image/jpeg' --data-binary @scene.jpg \
  "$base/v1/assets/upload?name=scene.jpg"
curl -fsS -H "Authorization: Bearer $IMAGE_API_TOKEN" \
  "$base/v1/assets/$asset_id/sign?w=800&h=800&fit=cover&format=webp&q=85"
```

签名包含路径、资产内容版本、规范化变换和到期时间。管理接口返回以图片域名为基准的相对路径。公开原件可使用资产 UUID 或 SHA-256，私有图由 UUID 签名读取。公开原图缓存一年并标记 immutable，公开衍生图缓存一天，私有图和管理响应使用 `private, no-store`。反代应绕过带 `exp` 或 `sig` 的请求，缓存键包含完整路径和变换查询串。

## 转换与资源

`fit=contain` 等比缩放，`cover` 居中裁切，`smart` 使用 libvips 注意力裁切，`fill` 强制尺寸。后三种都需要 `w` 和 `h`。可用 `crop=x,y,w,h` 先按方向修正后的源图坐标裁剪，再缩放。变体每边最多 4096。默认质量 JPEG/WebP 82，AVIF 55，AVIF 限 4 Mi 像素与质量 75。PNG 使用固定压缩。上传时记录 EXIF 方向，转换时应用方向并移除元数据。原图保持上传字节，发布前应核对原图内的元数据。

`preset=thumb` 生成 320×320 WebP，`preset=reader` 生成 1280×1920 范围内的 WebP。可用 `blur`、`sharpen` 和 `wm=<公开资产 UUID>` 调节效果或叠加右下水印。预热调用适合在公布阅读 URL 前执行。衍生图写入专用缓存，按源摘要、配方、编码版本和水印摘要去重。磁盘缓存按先写先淘汰，默认上限 2 GiB 和 10000 项。转换与解码在阻塞线程池共享 2 个许可证，libvips 默认每项使用 1 个工作线程。管理请求、转换未命中和上传分别限流或限并发。容量不足返回 429，可按 Retry-After 重试。

## AI

OpenAI 图片生成和编辑复用 Agent 的 `MEDIA_AI_*` 与 Magpie 图像路由约定，视觉分析使用 Agent 的 `VISION_AI_*` 和 Responses API。Gemini 需配置 `GEMINI_API_KEY` 和启用的视觉或图像模型。密钥从服务端环境读取。工作流及消费回执保存在本地 SQLite，现有 Agent 和 Gin 计费系统无需接入即可运行该服务。

分析请求传 `asset_id` 和可选描述提示。生成请求传 `prompt`，编辑传 `asset_id` 和 `prompt` 或 `task`。OpenAI 编辑可传同尺寸 PNG 的 `mask_id`。去背景和修复由生成模型按提示完成，结果质量和透明输出取决于所选模型。AI 输入单图限 8 MiB、1200 万像素、单边 4096，仅从资产目录读取。供应商出图通过同一套格式和尺寸验证，生成独立私有资产，编辑结果保留 `source_asset_id`。分析结果保存在任务 JSON 中。

单次上游请求限时 180 秒，最多尝试三次。429、连接失败和部分分析服务错误可重试。遇到出图响应超时、服务端结果不明或进程重启时，任务进入可查询的失败状态，以避免自动重复付费。未领取的排队任务继续运行。可用 `IMAGE_AI_WORKERS=0` 暂停领取任务。

## 备份与恢复

在线备份用 SQLite backup API 取一致性快照。备份脚本根据快照复制每个引用的原图并校验 SHA-256。可以将新目录放在独立备份卷，成功以 `COMPLETE` 为准。另行备份签名密钥，衍生缓存可重建。

```sh
docker compose build image
image=$(docker compose images -q image)
docker volume create folio-image-backups
# 仅初次初始化这个新建备份卷的所有者。
docker run --rm --user 0:0 --entrypoint chown \
  -v folio-image-backups:/backups "$image" 10001:10001 /backups
docker run --rm --entrypoint folio-image-backup \
  -v folio-image_image-data:/data:ro -v folio-image-backups:/backups \
  "$image" /backups/image-20261122
```

恢复到一份独立新卷。确认 `COMPLETE`，从备份根目录运行 `sha256sum -c SHA256SUMS`（非空清单）和 `sqlite3 library.sqlite 'PRAGMA integrity_check'`，复制 `library.sqlite` 和 `objects/`，将新卷文件归属设置为 UID/GID 10001。用原签名密钥启动隔离实例，核对就绪、资产详情、发布原图与短签名。切换入口前保留当前容器和原卷。在线备份挂载使用了已运行 SQLite 生成的 WAL/SHM，停服备份可先通过正常关闭完成 WAL 检查点。

## 验证与范围

本地恢复演练从在线快照还原了 100,003 条资产记录，三个原图摘要与备份清单一致。新卷启动后，原发布 UUID 的 HTTP 响应摘要与源文件相同，十万条目录的索引查询可用。

```sh
docker build --target test .
docker compose config --quiet
```

Docker test 阶段执行 rustfmt、Clippy 和 Rust 测试。S3/R2 仍待接入，存储接口已支持偏移读取和字节流。生产发布关系自动同步、自动回收孤立原图、断点续传、专用 AI 去背与修复服务、多租户权限以及模型账务联动属于后续扩展。
