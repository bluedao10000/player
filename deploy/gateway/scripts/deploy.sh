#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
exec 9>/var/lib/player-edge/deploy.lock
flock -x 9
root=/var/lib/player-edge/config
compose() { docker compose -p player-edge -f "$root/current/docker-compose.yml" "$@"; }
switch_to() {
    ln -s "$1" "$root/.next-$$"
    mv -Tf "$root/.next-$$" "$root/current"
}
activate() {
    before=$(compose ps -q --status running nginx) || return 1
    compose up -d nginx || return 1
    after=$(compose ps -q --status running nginx) || return 1
    if [ -n "$before" ] && [ "$before" = "$after" ]; then
        compose kill -s HUP nginx
    fi
}
previous=$(readlink "$root/current" || true)
if [ "$#" -eq 0 ]; then
    set -a
    . /etc/player-edge/settings
    set +a
    umask 022
    dest=$(mktemp -d "$root/release-XXXXXXXX")
    chmod 0755 "$dest"
    cp docker-compose.yml "$dest/docker-compose.yml"
    python3 scripts/render.py "$dest"
    release=${dest##*/}
elif [ "$#" -eq 2 ] && [ "$1" = --restore ]; then
    release=$2
    case "$release" in release-*) ;; *) exit 2;; esac
    case "$release" in *[!a-zA-Z0-9_-]*) exit 2;; esac
    test -f "$root/$release/nginx.conf"
else
    exit 2
fi
# nginx -t opens Unix listeners. Give validation its own runtime directory.
docker run --rm --network host --read-only --user 1000:1000 --tmpfs /tmp \
    --tmpfs /run/player-edge:uid=1000,gid=1000 -v "$root:/edge:ro" \
    --entrypoint nginx "$(docker compose -p player-edge -f "$root/$release/docker-compose.yml" config --images)" -t -c "/edge/$release/nginx.conf"
switch_to "$release"
if activate && scripts/healthcheck.sh "$release"; then
    echo "Active $release (previous: ${previous:-none})"
    exit 0
fi
if [ -n "$previous" ]; then
    switch_to "$previous"
    activate && scripts/healthcheck.sh "$previous"
else
    compose stop nginx
    rm -f "$root/current"
fi
exit 1
