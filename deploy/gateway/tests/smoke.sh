#!/bin/sh
# No ports published. HTTP clients and fake origins share an isolated Linux namespace.
set -eu
cd "$(dirname "$0")/.."
work=$(mktemp -d)
name=player-edge-test-$$
fixture=$name-origins
cleanup() {
    docker rm -f "$name" "$fixture" >/dev/null 2>&1 || true
    docker volume rm "$name" >/dev/null 2>&1 || true
    rm -rf "$work"
}
trap cleanup EXIT
mkdir "$work/test"
chmod 0755 "$work"
printf 'art/episode-scenes/%064d.jpg\n' 0 > "$work/manifest"
export MANIFEST="$work/manifest" PUBLIC_HOST=www.codeiy.com ACCOUNT_HOST=localhost
export WEB_UPSTREAM=127.0.0.1:18001 STATIC_UPSTREAM=127.0.0.1:18002 API_UPSTREAM=127.0.0.1:18003
export AUTH_UPSTREAM=127.0.0.1:18004 BILLING_UPSTREAM=127.0.0.1:18005
export IMAGE_UPSTREAM=127.0.0.1:18006 GRAY_UPSTREAM=127.0.0.1:18007 FALLBACK_UPSTREAM=127.0.0.1:18008
python3 scripts/render.py "$work/test"
ln -s test "$work/current"
docker volume create "$name" >/dev/null
docker run -d --network none --name "$fixture" -v "$name:/run/player-edge" \
    -v "$PWD/tests:/tests:ro" python:3-alpine sh -c 'chown 1000:1000 /run/player-edge && exec python /tests/probe.py serve' >/dev/null
nginx_image=$(docker compose -f docker-compose.yml config --images)
docker run --rm --network none --user 1000:1000 --tmpfs /run/player-edge:uid=1000,gid=1000 -v "$work:/edge:ro" \
    --entrypoint nginx "$nginx_image" -t -c /edge/test/nginx.conf
docker run -d --network "container:$fixture" --name "$name" --user 1000:1000 \
    -v "$work:/edge:ro" -v "$name:/run/player-edge" --entrypoint nginx "$nginx_image" \
    -c /edge/current/nginx.conf -g 'daemon off;' >/dev/null
docker exec "$fixture" python /tests/probe.py check

mkdir "$work/candidate"
python3 scripts/render.py "$work/candidate"
for release in candidate test; do
    python3 -c 'import os,sys; p,r=sys.argv[1:]; os.symlink(r,p+"/next"); os.replace(p+"/next",p+"/current")' "$work" "$release"
    # Wait for bind-mount propagation when Docker Desktop runs in a VM.
    docker exec "$name" sh -ec 'for i in 1 2 3 4 5; do cmp -s /edge/current/nginx.conf "/edge/$1/nginx.conf" && exit 0; sleep 1; done; exit 1' sh "$release"
    docker kill -s HUP "$name" >/dev/null
    docker exec "$fixture" python /tests/probe.py version "$release" || { docker logs "$name" 2>&1 | tail -60; exit 1; }
done
