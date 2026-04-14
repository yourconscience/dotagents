#!/usr/bin/env bash
# Wrapper for memsearch Claude Code hook scripts.
# Location: ~/.agents/bin/memsearch/hook.sh
# Sets env-var overrides so memsearch operates on a single global vault
# (~/Workspace/knowledge/ai) with a single shared Milvus collection ("ai"),
# regardless of the current project directory.
#
# Usage: bash memsearch-hook.sh <event>
# Events: session-start | user-prompt-submit | stop | session-end | pre-compact

set -eu

EVENT="${1:-}"
if [ -z "$EVENT" ]; then
  echo "usage: memsearch-hook.sh <event>" >&2
  exit 2
fi

# Prevent recursive Claude sessions from re-entering these hooks.
# The Stop hook shells out to `claude -p` for summarization; that child
# inherits this env var and its own SessionStart/Stop/SessionEnd hooks
# must no-op or Claude will recursively spawn more Claude processes.
if [ "${MEMSEARCH_SKIP_CLAUDE_HOOKS:-}" = "1" ]; then
  echo '{}'
  exit 0
fi

MEMSEARCH_SRC="$HOME/Public/memsearch/plugins/claude-code"
export MEMSEARCH_MEMORY_DIR="$HOME/Workspace/knowledge/ai"
export MEMSEARCH_STATE_DIR="$HOME/.memsearch/state"
export MEMSEARCH_COLLECTION_NAME="ai"
export CLAUDE_PLUGIN_ROOT="$MEMSEARCH_SRC"

mkdir -p "$MEMSEARCH_MEMORY_DIR" "$MEMSEARCH_STATE_DIR"

BACKFILL_BIN="$HOME/.agents/bin/memsearch/backfill/memsearch-backfill"

case "$EVENT" in
  session-start)
    # Lazy-consolidate any older daily files that are missing their Daily
    # digest header. Idempotent via --if-needed. Runs in background so we
    # don't block session startup.
    if [ -x "$BACKFILL_BIN" ]; then
      ("$BACKFILL_BIN" consolidate --old --if-needed >/dev/null 2>&1 &)
    fi
    exec bash "$MEMSEARCH_SRC/hooks/session-start.sh"
    ;;
  stop)
    # Any child `claude -p` launched by stop.sh should not run hooks again.
    export MEMSEARCH_SKIP_CLAUDE_HOOKS=1
    exec bash "$MEMSEARCH_SRC/hooks/stop.sh"
    ;;
  session-end)
    # Upstream cleanup first (kills watch/index, frees Milvus Lite lock).
    bash "$MEMSEARCH_SRC/hooks/session-end.sh" || true
    # Consolidate today's per-turn entries into session-level digests + daily
    # digest. Serialized after upstream cleanup - Milvus lock is free.
    if [ -x "$BACKFILL_BIN" ]; then
      "$BACKFILL_BIN" consolidate >/dev/null 2>&1 || true
    fi
    # Full-vault reindex so human notes + profile edits + the fresh digest
    # all get picked up by memsearch.
    exec memsearch index \
      "$HOME/Workspace/knowledge/notes" \
      "$HOME/Workspace/knowledge/profile" \
      "$HOME/Workspace/knowledge/ai" \
      --collection ai >/dev/null 2>&1
    ;;
  *)
    echo "unknown event: $EVENT" >&2
    exit 2
    ;;
esac
