#!/bin/sh
set -eu

usage() {
    echo "Usage: $0 docker|native SETTINGS" >&2
    exit 2
}

[ "$#" -eq 2 ] || usage
settings=$(cd "$(dirname "$2")" && pwd)/$(basename "$2")
mode=$1
cd "$(dirname "$0")/.."
set -a
. "$settings"
set +a

case "$mode" in
    docker)
        : "${EDGE_CONFIG_DIR:?rendered configuration directory}"
        python3 scripts/render.py "$EDGE_CONFIG_DIR" --docker
        docker compose --env-file "$settings" config --quiet
        docker compose --env-file "$settings" build nginx
        docker compose --env-file "$settings" run --rm --no-deps nginx -t -c /etc/folio-edge/nginx.conf
        docker compose --env-file "$settings" up -d --wait --force-recreate nginx
        docker compose --env-file "$settings" exec -T nginx -t -c /etc/folio-edge/nginx.conf
        ;;
    native)
        : "${EDGE_CONFIG_DIR:?active configuration directory}"
        : "${SITE_ROOT:=/srv/folio-site}"
        export SITE_ROOT
        (cd ../.. && npm ci && npm run build:web)
        python3 scripts/render.py "$EDGE_CONFIG_DIR"
        mkdir -p "$SITE_ROOT"
        nginx -t -c "$EDGE_CONFIG_DIR/nginx.conf"
        rsync -a --delay-updates --delete-delay ../../apps/web/dist/site/ "$SITE_ROOT"/
        if nginx -c "$EDGE_CONFIG_DIR/nginx.conf" -s reload; then
            exit 0
        fi
        nginx -c "$EDGE_CONFIG_DIR/nginx.conf"
        ;;
    *) usage ;;
esac
