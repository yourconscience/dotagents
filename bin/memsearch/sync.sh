#!/usr/bin/env bash
set -eu

CONF="${MEMSEARCH_CONF:-$HOME/.agents/memsearch.conf}"
if [ -f "$CONF" ]; then
  # shellcheck source=/dev/null
  . "$CONF"
fi

exec python3 "$HOME/.agents/bin/memsearch/sync.py" "$@"
