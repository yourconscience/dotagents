#!/usr/bin/env bash
set -eu

CONF="${MEMSEARCH_CONF:-$HOME/.agents/memsearch.conf}"
if [ -f "$CONF" ]; then
  # shellcheck source=/dev/null
  . "$CONF"
fi

export MEMSEARCH_VAULT_DIR="${MEMSEARCH_VAULT_DIR:-$HOME/Workspace/knowledge}"
export MEMSEARCH_AI_DIR="${MEMSEARCH_AI_DIR:-$MEMSEARCH_VAULT_DIR/ai}"
export MEMSEARCH_NOTES_DIR="${MEMSEARCH_NOTES_DIR:-$MEMSEARCH_VAULT_DIR/notes}"
export MEMSEARCH_PROFILE_DIR="${MEMSEARCH_PROFILE_DIR:-$MEMSEARCH_VAULT_DIR/profile}"
export MEMSEARCH_COLLECTION="${MEMSEARCH_COLLECTION:-ai}"

exec python3 "$HOME/.agents/bin/memsearch/finalize.py"
