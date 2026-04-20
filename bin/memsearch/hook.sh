#!/usr/bin/env bash
# Portable wrapper for memsearch Claude Code hooks.
# Reads config from ~/.agents/memsearch.conf (written by `dotagents memsearch setup`).
#
# Usage (in Claude Code hooks config):
#   bash ~/.agents/bin/memsearch/hook.sh <event>
# Events: session-start | stop | session-end

set -eu

EVENT="${1:-}"
if [ -z "$EVENT" ]; then
  echo "usage: memsearch-hook.sh <event>" >&2
  exit 2
fi

# Prevent recursive Claude sessions from re-entering these hooks.
if [ "${MEMSEARCH_SKIP_CLAUDE_HOOKS:-}" = "1" ]; then
  echo '{}'
  exit 0
fi

# Load config - exit gracefully if not configured
CONF="${MEMSEARCH_CONF:-$HOME/.agents/memsearch.conf}"
if [ ! -f "$CONF" ]; then
  exit 0
fi
# shellcheck source=/dev/null
. "$CONF"

# Resolve memsearch plugin directory
MEMSEARCH_PLUGIN_DIR="${MEMSEARCH_PLUGIN_DIR:-$(dirname "$(command -v memsearch 2>/dev/null || echo "")")/plugins/claude-code}"
if [ ! -d "$MEMSEARCH_PLUGIN_DIR" ]; then
  # Fallback: look for pip/uv installed plugin
  MEMSEARCH_PLUGIN_DIR="$(python3 -c 'import memsearch; import os; print(os.path.join(os.path.dirname(memsearch.__file__), "plugins", "claude-code"))' 2>/dev/null || echo "")"
fi
if [ ! -d "$MEMSEARCH_PLUGIN_DIR" ]; then
  exit 0
fi

# Export for memsearch plugin scripts
export MEMSEARCH_MEMORY_DIR="${MEMSEARCH_AI_DIR:-$MEMSEARCH_VAULT_DIR/ai}"
export MEMSEARCH_STATE_DIR="${MEMSEARCH_STATE_DIR:-$HOME/.memsearch/state}"
export MEMSEARCH_COLLECTION_NAME="${MEMSEARCH_COLLECTION:-ai}"

mkdir -p "$MEMSEARCH_MEMORY_DIR" "$MEMSEARCH_STATE_DIR"

case "$EVENT" in
  session-start)
    exec bash "$MEMSEARCH_PLUGIN_DIR/hooks/session-start.sh"
    ;;
  stop)
    export MEMSEARCH_SKIP_CLAUDE_HOOKS=1
    exec bash "$MEMSEARCH_PLUGIN_DIR/hooks/stop.sh"
    ;;
  session-end)
    bash "$MEMSEARCH_PLUGIN_DIR/hooks/session-end.sh" || true
    exec memsearch index \
      "${MEMSEARCH_NOTES_DIR:-$MEMSEARCH_VAULT_DIR/notes}" \
      "${MEMSEARCH_PROFILE_DIR:-$MEMSEARCH_VAULT_DIR/profile}" \
      "$MEMSEARCH_MEMORY_DIR" \
      --collection "$MEMSEARCH_COLLECTION_NAME" >/dev/null 2>&1
    ;;
  *)
    echo "unknown event: $EVENT" >&2
    exit 2
    ;;
esac
