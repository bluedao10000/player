#!/bin/sh
set -eu
umask 077
data=${IMAGE_DATA_DIR:?IMAGE_DATA_DIR is required}
dest=${1:?usage: folio-image-backup NEW_BACKUP_DIRECTORY}
case "$dest" in -*|*[!a-zA-Z0-9_./-]*) echo "Use an alphanumeric backup path" >&2; exit 2;; esac
mkdir "$dest"
trap 'echo "Backup incomplete: $dest" >&2' EXIT
# Originals survive catalog deletion. Use the same SQLite snapshot to select and verify objects.
test -f "$data/library.sqlite"
sqlite3 "$data/library.sqlite" ".timeout 5000" ".backup $dest/library.sqlite"
test "$(sqlite3 "$dest/library.sqlite" 'PRAGMA integrity_check')" = ok
sqlite3 -bail "$dest/library.sqlite" 'SELECT DISTINCT storage_key FROM assets ORDER BY storage_key' > "$dest/keys"
mkdir "$dest/objects"
while IFS= read -r key; do
    case "$key" in *[!a-f0-9]*) exit 1;; esac
    test "${#key}" -eq 64
    prefix=$(printf '%s' "$key" | cut -c1-2)
    shard=$(printf '%s' "$key" | cut -c3-4)
    directory=objects/$prefix/$shard
    mkdir -p "$dest/$directory"
    cp "$data/$directory/$key" "$dest/$directory/$key"
    printf '%s  %s/%s\n' "$key" "$directory" "$key"
done < "$dest/keys" > "$dest/SHA256SUMS"
rm "$dest/keys"
if [ -s "$dest/SHA256SUMS" ]; then
    (cd "$dest" && sha256sum -c SHA256SUMS >/dev/null)
fi
touch "$dest/COMPLETE"
trap - EXIT
