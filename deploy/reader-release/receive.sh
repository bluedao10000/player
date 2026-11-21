#!/bin/sh
# Installed root-owned as an SSH forced command. No caller arguments are used.
set -eu
root=/opt/codeiy/reader-release
archive=$(docker inspect player-online-api-1 --format '{{range .Mounts}}{{if eq .Destination "/archive"}}{{.Source}}{{end}}{{end}}')
test -d "$archive"
exec docker run --rm -i --init --user 501:501 --read-only --cap-drop=ALL --security-opt no-new-privileges \
 --network player-online_default --env-file "$root/receiver.env" -e KNOWLEDGE_ARCHIVE_DIR=/archive \
 -v "$archive:/archive" -v "$root/reader-release:/receiver:ro" --entrypoint /receiver postgres:17-alpine receive
