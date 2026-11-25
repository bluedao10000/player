#!/bin/sh
set -eu
cd "$(dirname "$0")/../../.."
work=$(mktemp -d)
prefix=folio-edge-test-$$
cleanup() {
    docker rm -f "$prefix-edge" "$prefix-reader" "$prefix-gin" "$prefix-image" >/dev/null 2>&1 || true
    docker network rm "$prefix" >/dev/null 2>&1 || true
    rm -rf "$work"
}
trap cleanup EXIT HUP INT TERM

if [ "${EDGE_SMOKE_SKIP_BUILD:-0}" != 1 ]; then
    docker build --target site -t folio-edge-cutover-test -f apps/web/Dockerfile .
    docker build --target reader-api -t folio-reader-api-cutover-test -f apps/web/Dockerfile .
    docker build -t folio-image-cutover-test apps/image
fi
openssl req -x509 -newkey rsa:2048 -nodes -sha256 -days 1 -subj /CN=www.codeiy.com \
    -addext 'subjectAltName=DNS:www.codeiy.com,DNS:img.codeiy.com' \
    -keyout "$work/privkey.pem" -out "$work/fullchain.pem" 2>/dev/null
API_UPSTREAM=gin-fixture:8080 READER_API_UPSTREAM=reader-api:4310 IMAGE_UPSTREAM=image:14580 \
    IMAGE_UPSTREAM_HOST=image ACCOUNT_HOST=gin-fixture \
    python3 deploy/gateway/scripts/render.py "$work" --docker

docker network create --subnet 172.31.250.0/24 "$prefix" >/dev/null
docker run -d --name "$prefix-gin" --network "$prefix" --ip 172.31.250.4 --network-alias gin-fixture \
    -v "$PWD/deploy/gateway/tests/fixture.py:/fixture.py:ro" python:3-alpine python /fixture.py >/dev/null
docker run -d --name "$prefix-reader" --network "$prefix" --ip 172.31.250.5 --network-alias reader-api \
    -e KNOWLEDGE_API_URL=http://gin-fixture:8080 -e READER_API_TRUSTED_PROXY_IP=172.31.250.3 \
    folio-reader-api-cutover-test >/dev/null
docker run -d --name "$prefix-image" --network "$prefix" --ip 172.31.250.6 --network-alias image -p 127.0.0.1::14580 \
    -e IMAGE_API_TOKEN=test-token-of-at-least-thirty-two-characters \
    -e IMAGE_SIGNING_KEY_HEX=4242424242424242424242424242424242424242424242424242424242424242 \
    -e IMAGE_AI_WORKERS=0 folio-image-cutover-test >/dev/null
docker run -d --name "$prefix-edge" --network "$prefix" --ip 172.31.250.3 -p 127.0.0.1::80 -p 127.0.0.1::443 \
    -v "$work:/etc/folio-edge:ro" -v "$work:/etc/folio-tls:ro" \
    --entrypoint nginx folio-edge-cutover-test -c /etc/folio-edge/nginx.conf -g 'daemon off;' >/dev/null

port() { docker port "$prefix-$1" "$2" | cut -d: -f2; }
for _ in $(seq 1 20); do
    if docker exec "$prefix-image" curl -fsS http://127.0.0.1:14580/readyz >/dev/null 2>&1 &&
       docker exec "$prefix-edge" nginx -t -c /etc/folio-edge/nginx.conf; then
        break
    fi
    sleep 1
done
docker exec "$prefix-edge" nginx -t -c /etc/folio-edge/nginx.conf
python3 deploy/gateway/tests/probe.py "$(port edge 80)" "$(port edge 443)" "$(port image 14580)" "$work/fullchain.pem"
if [ "${EDGE_SMOKE_BROWSER:-0}" = 1 ]; then
    : > "$work/references"
    IMAGE_ADMIN_URL="http://127.0.0.1:$(port image 14580)" \
        IMAGE_API_TOKEN=test-token-of-at-least-thirty-two-characters \
        python3 deploy/gateway/scripts/seed-images.py "$work/references" --scene-archive "$work" --portrait-archive "$work"
    node deploy/gateway/tests/browser.cjs "$(port edge 443)"
fi
docker stop -t 2 "$prefix-image" >/dev/null
code=$(curl --noproxy '*' --resolve "img.codeiy.com:$(port edge 443):127.0.0.1" --cacert "$work/fullchain.pem" -sS -o /dev/null -w '%{http_code}' \
    "https://img.codeiy.com:$(port edge 443)/i/$(printf '%064d' 0)")
test "$code" = 503
printf '%s\n' 'PASS image upstream failure returns 503'
