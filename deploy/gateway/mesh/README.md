# 私有中继

杭州 Nginx 的 host 网络不能直接使用 Mesh 容器的路由。杭州三个客户端在 mesh 网络命名空间建立 /mesh/{web,api,image}.sock，广州监听器在自己的 Mesh IP 上分三个端口接收，业务容器使用 Docker DNS。旧 14580 灰度由原 images.yaml 提供。[原连接器](../../cloudflare/mesh/compose.yaml) · [现网图片中继](../../cloudflare/mesh/images.yaml)

1. 在节点的受限目录准备现有 Mesh 项目的 compose.yaml、mesh-node.env、.env，沿用实际 Compose 项目名、外部 transport 网络和注册卷。在杭州 /etc/player-mesh 使用同一份安装配置，或者用 systemd override 指向已注册项目。检查 `docker compose ls`、容器 Mounts 和注册身份，避免启动第二个同名身份。安装程序不复制注册凭据。
2. 杭州和广州私有 .env 填各自已登记的 ALI_MESH_IP、GZ_MESH_IP，选定 MESH_WEB_PORT、MESH_API_PORT、MESH_IMAGE_PORT 三个未被占用的非特权 TCP 端口；两端保持一致，与旧图片、发布及备份监听器分开。Cloudflare 设备路由只包含相应 /32，私网策略按源身份、源 /32、目标 /32 和上述端口逐条放行，然后默认拒绝。现有 configure-peer.py 只覆盖旧 release/image/backup 三条，不能代替新增规则。
3. 广州建立仅供此入口使用的用户自定义 Docker 网桥，指定经过 IPAM 核对的 CIDR。GZ_EDGE_NETWORK 和 GZ_EDGE_BRIDGE_IP 与广州业务 Compose 完全一致。将 guangzhou.yaml、reader-origin.conf.template 放入现有 Mesh Compose 文件所在目录。模型 relay 另需 SV_MESH_IP、SV_MODEL_PORT 和由 UID/GID 1000 管理的 MODEL_SOCKET_DIR（0750）。模型证书由独立身份维护。
4. 先在广州用 `docker compose -f compose.yaml -f guangzhou.yaml config --quiet` 验证，在测试身份或维护窗口启动 mesh、reader-origin、model-client。**给现网 mesh 增加网桥可能重建网络命名空间**。保留当前全部 overlay 和 profile，记录并重启依附它的旧监听器，再验证原灰度、备份和发布路径。先做旁路时可使用独立注册的预发 Mesh 身份、项目和网桥，规则及运行时间与正式身份分开。
5. 杭州完成同样的 Compose 校验后启用 player-edge-mesh.service。它由 systemd 管理项目生命周期，容器自身带重启策略；停止单元只停三个新 socket，不会停止共用的 mesh。重启现网连接器按上一条处理所有依赖。分别从杭州 socket 取 HTML、discovery JSON 和一张批准的 /i/ 原图。从其他已登记节点验证三端口拒绝，从授权源请求 API 管理路径也应返回 404。
6. 硅谷在已登记 Mesh 命名空间使用 [TLS 入口模板](../../model-gateway/mesh-ingress.conf.template)，绑定 SV_MESH_IP:SV_MODEL_PORT，后端 SV_PRIVATE_MODEL_UPSTREAM 为同节点私有模型网关。渲染时 envsubst 指定模板中的三个地址变量及 GZ_MESH_IP，保留 Nginx 的 $proxy_host。仅接受专用广州客户端 CA 签发的有效证书，网关再校验每个调用方的短期 token、模型与配额。试无客户端证书、错误模型、错误来源和正确凭据，验证拒绝和允许。供应商 HTTPS 域名在硅谷出站白名单内。
