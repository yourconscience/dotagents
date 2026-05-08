#!/usr/bin/env bash
set -eu

. "$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)/common.sh"
load_memory_config

exec python3 "$MEMORY_DIR/lib/sync.py" "$@"
