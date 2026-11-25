# 私有中继

杭州 relay 在 Mesh 网络命名空间监听 /mesh/{reader-api,api,image}.sock。广州在指定私网地址接收三个端口的流量，按源 /32 放行，并用 Docker DNS 连接三项业务服务。[连接器](../../cloudflare/mesh/compose.yaml)

1. 在节点的受限目录准备现有 Mesh 项目的 compose.yaml、mesh-node.env、.env，沿用实际 Compose 项目名、外部 transport 网络和注册卷。在杭州 /etc/player-mesh 使用同一份安装配置，或者用 systemd override 指向已注册项目。检查 `docker compose ls`、容器 Mounts 和注册身份，避免启动第二个同名身份。安装程序不复制注册凭据。
2. 两端填写已登记的 ALI_MESH_IP、GZ_MESH_IP，明确 MESH_READER_API_PORT、MESH_API_PORT、MESH_IMAGE_PORT。Gateway 按登记身份、源 /32、目标 /32 和端口逐条放行，其余默认拒绝。端口按实际 IPAM 和网桥分配。
3. 广州建立仅供此入口使用的用户自定义 Docker 网桥，指定经过 IPAM 核对的 CIDR。GZ_EDGE_NETWORK 和 GZ_EDGE_BRIDGE_IP 与广州业务 Compose 完全一致。将 guangzhou.yaml、reader-origin.conf.template 放入现有 Mesh Compose 文件所在目录。模型 relay 另需 SV_MESH_IP、SV_MODEL_PORT 和由 UID/GID 1000 管理的 MODEL_SOCKET_DIR（0750）。模型证书由独立身份维护。
4. 在广州运行 `docker compose -f compose.yaml -f guangzhou.yaml config --quiet`，然后启动 mesh、reader-origin、model-client。修改网桥成员资格可能重建 Mesh 网络命名空间，应在维护窗口联动重启依附服务，并复验发布与备份。
5. 杭州通过 Compose 校验后启用 player-edge-mesh.service。socket 由 UID 1000 访问。用 `curl --unix-socket` 检查 Reader API 健康、discovery JSON 及 `/i/` 原图。非授权节点访问应被拒绝。将三个 socket 填入网关 settings，并挂载中继目录。
6. 硅谷在已登记 Mesh 命名空间使用 [TLS 入口模板](../../model-gateway/mesh-ingress.conf.template)，绑定 SV_MESH_IP:SV_MODEL_PORT，后端 SV_PRIVATE_MODEL_UPSTREAM 为同节点私有模型网关。渲染时 envsubst 指定模板中的三个地址变量及 GZ_MESH_IP，保留 Nginx 的 $proxy_host。仅接受专用广州客户端 CA 签发的有效证书，网关再校验每个调用方的短期 token、模型与配额。试无客户端证书、错误模型、错误来源和正确凭据，验证拒绝和允许。供应商 HTTPS 域名在硅谷出站白名单内。
