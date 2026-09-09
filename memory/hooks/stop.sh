#!/usr/bin/env bash
set -eu

. "$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)/common.sh"

if [ "${MEMSEARCH_SKIP_CLAUDE_HOOKS:-}" = "1" ]; then
  echo '{}'
  exit 0
fi

load_memory_config

payload="$(mktemp)"
trap 'rm -f "$payload"' EXIT
cat >"$payload"

# Codex/OMP capture may be wired to their Stop event (they expose no reliable
# session-end hook); route those to the local basic_memory digest. Claude Stop
# fires once per response, so it stays a clean continuation and lets the Claude
# SessionEnd hook own the full-session digest.
kind="$(classify_payload "$payload")"
case "$kind" in
  codex|omp)
    dispatch_basic_digest "$payload"
    exit 0
    ;;
esac

plugin_dir="$(resolve_claude_memory_plugin || true)"
if [ -z "$plugin_dir" ]; then
  printf '{"continue":true,"suppressOutput":true}\n'
  exit 0
fi

prepare_memory_index_env
export MEMSEARCH_SKIP_CLAUDE_HOOKS=1
exec bash "$plugin_dir/hooks/stop.sh" <"$payload"
