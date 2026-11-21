#!/bin/sh
# The snapshot owner stays connected until pg_dump and the source counts finish.
set -eu
umask 077
export PGCONNECT_TIMEOUT=15
if [ "${1:-}" = dump ]; then
    exec pg_dump --format=custom --snapshot="$FOLIO_SNAPSHOT" --file="$RUN_DIR/folio.dump"
fi
exec 9>/snapshots/.lock
flock -n 9 || { echo '另一个备份正在运行'; exit 1; }
stamp=$(date -u +%Y%m%dT%H%M%SZ)
RUN_DIR=$(mktemp -d /snapshots/.partial.XXXXXXXX)
export RUN_DIR
trap 'rm -rf "$RUN_DIR"' EXIT
(
  export PGHOST=/source PGDATABASE=folio PGUSER=folio_backup_read
  PGPASSWORD=$(cat /secrets/source-password)
  export PGPASSWORD
  psql -XqAt -v ON_ERROR_STOP=1 > "$RUN_DIR/counts.tsv" <<'SQL'
BEGIN ISOLATION LEVEL REPEATABLE READ READ ONLY;
SELECT pg_export_snapshot() AS snapshot \gset
\setenv FOLIO_SNAPSHOT :snapshot
\i /scripts/counts.sql
\! /scripts/backup.sh dump
\if :SHELL_ERROR
SELECT 1/0;
\endif
COMMIT;
SQL
)
(cd "$RUN_DIR" && sha256sum folio.dump > SHA256SUMS && sha256sum -c SHA256SUMS)
export PGHOST=postgres PGDATABASE=postgres PGUSER=postgres
PGPASSWORD=$(cat /secrets/target-password)
export PGPASSWORD
# These names are reserved for rotation in this dedicated backup cluster.
dropdb --if-exists folio_staging
createdb --template=template0 folio_staging
pg_restore --exit-on-error --single-transaction --no-owner --no-acl --dbname=folio_staging "$RUN_DIR/folio.dump"
psql -XqAt -v ON_ERROR_STOP=1 -d folio_staging -f /scripts/counts.sql > "$RUN_DIR/restored.tsv"
cmp "$RUN_DIR/counts.tsv" "$RUN_DIR/restored.tsv"
dropdb --if-exists folio_previous
psql -Xq -v ON_ERROR_STOP=1 <<'SQL'
BEGIN;
SELECT EXISTS(SELECT FROM pg_database WHERE datname='folio_backup') AS previous \gset
\if :previous
ALTER DATABASE folio_backup RENAME TO folio_previous;
\endif
ALTER DATABASE folio_staging RENAME TO folio_backup;
COMMIT;
SQL
mv "$RUN_DIR" "/snapshots/$stamp"
# Keep the seven newest verified snapshots, even after a skipped day.
ls -1d /snapshots/????????T??????Z | sort -r | tail -n +8 | while read -r expired; do rm -r "$expired"; done
echo "verified $stamp $(du -h "/snapshots/$stamp/folio.dump" | cut -f1)"
cat "/snapshots/$stamp/counts.tsv"
