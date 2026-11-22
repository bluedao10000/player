# 图片服务性能

2026-11-22 在 Apple M5 的 Docker Desktop Linux arm64 容器，用 release 镜像实测。容器限 4 vCPU、8 GiB，libvips 8.16.1，解码与转换并发 2，libvips 每项 1 线程。客户端在宿主机经回环读取完整响应，使用连接复用。该环境的虚拟磁盘、端口转发和 CPU 与广州生产机不同。

## 样本

| 图 | 字节 | 尺寸 |
| --- | ---: | ---: |
| 仓库 `keeper.webp` | 95,966 | 740 × 742 |
| 仓库 `night-post.webp` | 409,818 | 1672 × 941 |
| NASA 同框天文图，NASA 图片服务按 4300 像素宽输出 PNG | 18,500,112 | 4300 × 2076 |

NASA 样本 SHA-256 为 `bafcc2e64636dd65bd50dea09f334a197d39f5c02db120f8edae382ec1da7610`。目录用同一真实原图生成 1 万和 10 万条独立资产记录。分页每次返回 50 条，元数据搜索在各规模只命中一个匹配对象，搜索词至少三个字符并走 trigram 索引。容量测试只针对目录。 [NASA 样本来源](https://science.nasa.gov/asset/webb/pillars-of-creation-hubble-and-webb-images-side-by-side/)

## 结果

| 场景 | 请求 / 并发 | QPS | P50 ms | P95 ms | P99 ms | CPU（单核 = 100%） |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| 小图原件 | 500 / 8 | 2,573.76 | 2.787 | 4.534 | 5.422 | 94.00% |
| 普通图原件 | 500 / 8 | 721.28 | 10.725 | 15.556 | 17.072 | 52.17% |
| 18.5 MB 原件 | 80 / 8 | 12.98 | 642.137 | 730.683 | 778.152 | 24.86% |
| 盘缓存命中 320 px 变体 | 600 / 8 | 6,570.48 | 1.080 | 2.033 | 2.395 | 194.39% |
| 4300 px 源图即时 WebP | 32 / 2 | 12.60 | 151.852 | 216.064 | 222.535 | 199.76% |
| 4300 px 源图即时 AVIF | 12 / 2 | 14.76 | 134.789 | 142.612 | 142.612 | 200.58% |
| 1 万条分页 | 300 / 8 | 3,333.50 | 2.302 | 3.320 | 4.171 | 368.08% |
| 1 万条元数据搜索 | 120 / 4 | 6,702.88 | 0.492 | 0.744 | 2.654 | 178.07% |
| 10 万条分页 | 300 / 8 | 3,449.89 | 2.244 | 3.223 | 3.780 | 361.22% |
| 10 万条元数据搜索 | 120 / 4 | 6,956.10 | 0.494 | 0.706 | 1.985 | 182.42% |

- 预热缩略图 600/600 命中。两场首次转换合计 44 次未命中，WebP 32 次与 AVIF 12 次。转换延迟包含源图读取、解码、变换、编码、缓存落盘和 HTTP 传输。
- cgroup 内存峰值在 WebP 场景结束时为 344.77 MiB，AVIF 场景结束时为 351.42 MiB，整轮为 463.30 MiB。数值包含服务进程、SQLite 造数进程和文件页缓存，是累计峰值。
- 测量窗口内各场景的 cgroup 块设备读写增量为 0。整轮结束后，容器累计读取 839,680 字节、写入 137,875,456 字节，包含 SQLite 造数和延迟回写。卷内约有 19 MiB 原图、2.6 MiB 变体和 55 MiB SQLite 主文件。结果不能用于估计冷盘吞吐。
- 短于三个字符的查询仍扫描筛选后的目录。引入索引前，同一条 10 万目录的稀疏元数据搜索 P95 为 236.80 ms。短词和高匹配率词的生产分布仍需现场测量。

## 复测

启动单独的新容器和空命名卷，设置 `--cpus 4 --memory 8g`、`IMAGE_API_RPS=5000` 和 `IMAGE_TRANSFORM_RPS=5000`，并映射回环端口。吞吐测试临时调高应用限流，转换并发使用默认值。脚本会发布测试图并直接添加 100,000 条 SQLite 测试记录。

```sh
curl -fsSL 'https://assets.science.nasa.gov/dynamicimage/assets/science/missions/webb/science/2022/10/STScI-01GFNQZAFP4YXE5X0JB9SXBFD9.png?w=4300&h=2078&fit=clip' \
  -o /tmp/pillars-4300.png
# 用该测试容器的 IMAGE_API_TOKEN 调用脚本。
python3 apps/image/ops/benchmark.py --container folio-image-bench \
  --small apps/web/public/art/keeper.webp \
  --medium apps/web/public/art/night-post.webp \
  --large /tmp/pillars-4300.png
```
