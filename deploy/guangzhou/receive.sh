#!/bin/sh
# Install root-owned with a fixed checkout and env file; use only as a forced command.
set -eu
cd /opt/player
exec docker compose --env-file /etc/player-gz/settings -f deploy/guangzhou/compose.yaml \
    run --rm --no-deps -T release-receiver
