#!/bin/sh
# /snapshots is a dedicated backup PVC. "source" is a locked-down SSH config alias.
set -eu
umask 077
exec 9>/snapshots/.image-lock
flock -n 9 || { echo "Image restore already running" >&2; exit 1; }
stage=$(mktemp -d /snapshots/.incoming.XXXXXXXX)
restore=$(mktemp -d)
pid=
cleanup() {
    if [ -n "$pid" ]; then kill "$pid" 2>/dev/null || true; wait "$pid" || true; fi
    rm -rf "$stage" "$restore"
}
trap cleanup EXIT
export RSYNC_RSH='ssh -F /ssh/config -oBatchMode=yes -oStrictHostKeyChecking=yes'
rsync -rt --timeout=120 source:LATEST "$stage/LATEST"
id=$(cat "$stage/LATEST")
echo "$id" | grep -Exq 'image-[0-9]{8}T[0-9]{6}Z'
# Reject embedded newlines as well as path separators.
test "${#id}" = 22
if [ -d "/snapshots/$id" ]; then
    cd "/snapshots/$id"
else
    rsync -rt --timeout=180 "source:$id/" "$stage/"
    cd "$stage"
fi
test -f COMPLETE
test "$(sqlite3 -readonly library.sqlite 'PRAGMA integrity_check')" = ok
sqlite3 -readonly library.sqlite 'SELECT DISTINCT storage_key FROM assets ORDER BY storage_key' > "$restore/keys"
while IFS= read -r key; do
    case "$key" in *[!a-f0-9]*) exit 1;; esac
    test "${#key}" = 64
    printf '%s  objects/%.2s/%.2s/%s\n' "$key" "$key" "$(echo "$key" | cut -c3-4)" "$key"
done < "$restore/keys" > "$restore/expected"
cmp "$restore/expected" SHA256SUMS
if [ -s SHA256SUMS ]; then sha256sum --check --quiet SHA256SUMS; fi
# Boot the recovered catalog with synthetic credentials and no background AI calls.
cp library.sqlite "$restore/library.sqlite"
ln -s "$PWD/objects" "$restore/objects"
export IMAGE_DATA_DIR="$restore" IMAGE_BIND=127.0.0.1:14580 IMAGE_AI_WORKERS=0
IMAGE_API_TOKEN=$(od -An -N32 -tx1 /dev/urandom | tr -d ' \n')
IMAGE_SIGNING_KEY_HEX=$(od -An -N32 -tx1 /dev/urandom | tr -d ' \n')
export IMAGE_API_TOKEN IMAGE_SIGNING_KEY_HEX
folio-image &
pid=$!
for attempt in $(seq 1 30); do
    if curl -fsS http://127.0.0.1:14580/readyz >/dev/null 2>&1; then break; fi
    kill -0 "$pid"
    sleep 1
done
curl -fsS http://127.0.0.1:14580/readyz >/dev/null
asset=$(sqlite3 -readonly library.sqlite 'SELECT id FROM assets WHERE deleted_at IS NULL ORDER BY id LIMIT 1')
if [ -n "$asset" ]; then
    url=$(curl -fsS -H "Authorization: Bearer $IMAGE_API_TOKEN" "http://127.0.0.1:14580/v1/assets/$asset/sign" | jq -er .url)
    curl -fsS "http://127.0.0.1:14580$url" -o "$restore/sample"
    checksum=$(sha256sum "$restore/sample" | cut -d' ' -f1)
    test "$(sqlite3 -readonly library.sqlite 'SELECT sha256 FROM assets WHERE deleted_at IS NULL ORDER BY id LIMIT 1')" = "$checksum"
fi
kill "$pid"
wait "$pid"
pid=
cd /snapshots
if [ ! -d "$id" ]; then mv -T "$stage" "$id"; fi
# Rotate only verified snapshot directories in this dedicated PVC.
ls -1d image-????????T??????Z | sort -r | tail -n +8 | while read -r expired; do
    test -f "$expired/COMPLETE" && rm -r -- "$expired"
done
echo "verified $id"
