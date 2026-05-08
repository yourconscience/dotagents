#!/usr/bin/env bash
set -eu

. "$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)/common.sh"

if [ "${MEMSEARCH_SKIP_CLAUDE_HOOKS:-}" = "1" ]; then
  echo '{}'
  exit 0
fi

load_memory_config
plugin_dir="$(resolve_claude_memory_plugin || true)"
[ -n "$plugin_dir" ] || exit 0

prepare_memory_index_env
export MEMSEARCH_SKIP_CLAUDE_HOOKS=1
exec bash "$plugin_dir/hooks/stop.sh"
