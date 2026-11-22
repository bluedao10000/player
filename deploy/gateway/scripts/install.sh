#!/bin/sh
# Run from a released checkout on Linux. Install files only; activation is separate.
set -eu
cd "$(dirname "$0")/.."
getent group player-edge >/dev/null || groupadd --system player-edge
install -d /opt/player-edge /etc/player-edge /etc/player-mesh
install -d -m 2750 -g 1000 /var/lib/player-edge/config
install -d -m 0700 -o 1000 -g 1000 /var/lib/player-edge/mesh
cp -R nginx scripts docker-compose.yml /opt/player-edge/
install -m 0644 cloudflare/player-edge-tunnel.service mesh/player-edge-mesh.service /etc/systemd/system/
install -m 0644 mesh/aliyun.yaml /etc/player-mesh/aliyun.yaml
install -m 0644 cloudflare/tunnel.yml.in /etc/player-edge/tunnel.yml.in
test -e /etc/player-edge/settings || install -m 0600 .env.example /etc/player-edge/settings
echo 'd /run/player-edge 2750 1000 player-edge -' > /etc/tmpfiles.d/player-edge.conf
systemd-tmpfiles --create /etc/tmpfiles.d/player-edge.conf
systemctl daemon-reload
