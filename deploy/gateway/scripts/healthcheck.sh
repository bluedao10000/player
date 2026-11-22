#!/bin/sh
set -eu
if [ "${1:-}" = --upstreams ]; then
    image=${2:?supply a published /i/UUID for a read-only probe}
    echo "$image" | grep -Eq '^/i/[0-9a-fA-F-]{36}$' || exit 2
    host=$(cat /var/lib/player-edge/config/current/public-host)
    probe() {
        curl --max-time 12 --silent --show-error --unix-socket /run/player-edge/origin.sock -H "Host: $host" "$@"
    }
    for path in /healthz /api/v1/knowledge/discovery "$image"; do
        probe --fail --output /dev/null "http://origin$path"
    done
    test "$(probe --output /dev/null --write-out '%{http_code}' http://origin/workbench)" = 404
    exit
fi
release=${1:-$(readlink /var/lib/player-edge/config/current)}
attempt=0
while [ "$attempt" -lt 10 ]; do
    if [ "$(curl -fsS --max-time 2 --unix-socket /run/player-edge/check.sock http://localhost/ 2>/dev/null)" = "$release" ]; then
        exit 0
    fi
    attempt=$((attempt + 1))
    sleep 1
done
echo "Gateway did not load $release" >&2
exit 1
