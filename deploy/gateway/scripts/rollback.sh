#!/bin/sh
set -eu
exec "$(dirname "$0")/deploy.sh" --restore "${1:?supply the last accepted release ID}"
