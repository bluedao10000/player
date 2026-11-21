#!/usr/bin/env bash
set -euo pipefail
cd /srv/folio-data
case "$1" in
  inventory)
    umask 077
    mkdir -p inventory-outbox
    snapshot=$(mktemp inventory-outbox/.collect.XXXXXXXX)
    trap 'rm -f "$snapshot"' EXIT
    EPISODE_CATALOG_DIR="$PWD/catalog" python3 inventory.py >"$snapshot"
    mv "$snapshot" "inventory-outbox/$(date -u +%Y%m%dT%H%M%SZ).json"
    if [[ -z "${DATABASE_URL:-}" ]]; then
      echo 'Inventory captured locally, waiting for database connection'
      exit 0
    fi
    for pending in inventory-outbox/*.json; do
      bin/knowledge episode-inventory-import <"$pending"
      rm "$pending"
    done
    ;;
  evidence)
    while true; do
      job=$(bin/knowledge episode-capture-claim)
      [[ -n "$job" ]] || break
      sudo -n docker run -i --rm --pull never --network host --cap-drop=ALL --read-only --user node \
        -v "$PWD/collection:/collection:ro" -v "$PWD/agent-src:/app/apps/agent/src:ro" folio/pi:phase1 \
        node /collection/capture.mjs <<< "$job" | bin/knowledge episode-evidence-import
    done
    ;;
  *) exit 2 ;;
esac
