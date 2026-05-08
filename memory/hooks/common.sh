#!/usr/bin/env bash

MEMORY_HOOK_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
MEMORY_DIR="$(dirname "$MEMORY_HOOK_DIR")"

load_memory_config() {
  CONF="${MEMSEARCH_CONF:-$HOME/.agents/memsearch.conf}"
  MEMSEARCH_VAULT_DIR_ENV="${MEMSEARCH_VAULT_DIR:-}"
  MEMSEARCH_AI_DIR_ENV="${MEMSEARCH_AI_DIR:-}"
  MEMSEARCH_NOTES_DIR_ENV="${MEMSEARCH_NOTES_DIR:-}"
  MEMSEARCH_PROFILE_DIR_ENV="${MEMSEARCH_PROFILE_DIR:-}"
  MEMSEARCH_STATE_DIR_ENV="${MEMSEARCH_STATE_DIR:-}"
  MEMSEARCH_COLLECTION_ENV="${MEMSEARCH_COLLECTION:-}"
  if [ -f "$CONF" ]; then
    # shellcheck source=/dev/null
    . "$CONF"
  fi

  export MEMSEARCH_VAULT_DIR="${MEMSEARCH_VAULT_DIR_ENV:-${MEMSEARCH_VAULT_DIR:-$HOME/Workspace/knowledge}}"
  export MEMSEARCH_AI_DIR="${MEMSEARCH_AI_DIR_ENV:-${MEMSEARCH_AI_DIR:-$MEMSEARCH_VAULT_DIR/ai}}"
  export MEMSEARCH_NOTES_DIR="${MEMSEARCH_NOTES_DIR_ENV:-${MEMSEARCH_NOTES_DIR:-$MEMSEARCH_VAULT_DIR/notes}}"
  export MEMSEARCH_PROFILE_DIR="${MEMSEARCH_PROFILE_DIR_ENV:-${MEMSEARCH_PROFILE_DIR:-$MEMSEARCH_VAULT_DIR/profile}}"
  export MEMSEARCH_STATE_DIR="${MEMSEARCH_STATE_DIR_ENV:-${MEMSEARCH_STATE_DIR:-$HOME/.memsearch/state}}"
  export MEMSEARCH_COLLECTION="${MEMSEARCH_COLLECTION_ENV:-${MEMSEARCH_COLLECTION:-ai}}"
}

resolve_claude_memory_plugin() {
  plugin_dir="${MEMSEARCH_PLUGIN_DIR:-$(dirname "$(command -v memsearch 2>/dev/null || echo "")")/plugins/claude-code}"
  if [ ! -d "$plugin_dir" ]; then
    plugin_dir="$(python3 -c 'import memsearch; import os; print(os.path.join(os.path.dirname(memsearch.__file__), "plugins", "claude-code"))' 2>/dev/null || echo "")"
  fi
  [ -d "$plugin_dir" ] && printf '%s\n' "$plugin_dir"
}

prepare_memory_index_env() {
  export MEMSEARCH_MEMORY_DIR="${MEMSEARCH_AI_DIR}"
  export MEMSEARCH_COLLECTION_NAME="${MEMSEARCH_COLLECTION}"
  mkdir -p "$MEMSEARCH_MEMORY_DIR" "$MEMSEARCH_STATE_DIR"
}

index_memory_top_level() {
  set -- "$MEMSEARCH_NOTES_DIR" "$MEMSEARCH_PROFILE_DIR"
  for path in "$MEMSEARCH_AI_DIR"/*.md "$MEMSEARCH_AI_DIR"/*.markdown; do
    [ -f "$path" ] && set -- "$@" "$path"
  done
  memsearch index "$@" --collection "$MEMSEARCH_COLLECTION" >/dev/null 2>&1
}
