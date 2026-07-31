#!/usr/bin/env bash
set -eu

. "$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)/common.sh"

if [ "${MEMSEARCH_SKIP_CLAUDE_HOOKS:-}" = "1" ]; then
  echo '{}'
  exit 0
fi

load_memory_config
plugin_dir="$(resolve_claude_memory_plugin || true)"
if [ -z "$plugin_dir" ]; then
  printf '{"continue":true,"suppressOutput":true}\n'
  exit 0
fi

prepare_memory_index_env
exec bash "$plugin_dir/hooks/session-start.sh"
