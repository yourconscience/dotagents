#!/usr/bin/env bash

MEMORY_HOOK_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
MEMORY_DIR="$(dirname "$MEMORY_HOOK_DIR")"

load_memory_config() {
  CONF="${KNOWLEDGE_CONF:-$HOME/.agents/memsearch.conf}"
  KNOWLEDGE_DIR_ENV="${KNOWLEDGE_DIR:-}"
  SESSIONS_DIR_ENV="${SESSIONS_DIR:-}"
  NOTES_DIR_ENV="${NOTES_DIR:-}"
  PROFILE_DIR_ENV="${PROFILE_DIR:-}"
  MEMSEARCH_STATE_DIR_ENV="${MEMSEARCH_STATE_DIR:-}"
  MEMSEARCH_COLLECTION_ENV="${MEMSEARCH_COLLECTION:-}"
  if [ -f "$CONF" ]; then
    # shellcheck source=/dev/null
    . "$CONF"
  fi

  export KNOWLEDGE_DIR="${KNOWLEDGE_DIR_ENV:-${KNOWLEDGE_DIR:-$HOME/Workspace/knowledge}}"
  export SESSIONS_DIR="${SESSIONS_DIR_ENV:-${SESSIONS_DIR:-$KNOWLEDGE_DIR/sessions}}"
  export NOTES_DIR="${NOTES_DIR_ENV:-${NOTES_DIR:-$KNOWLEDGE_DIR/notes}}"
  export PROFILE_DIR="${PROFILE_DIR_ENV:-${PROFILE_DIR:-$KNOWLEDGE_DIR/profile}}"
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
  export MEMSEARCH_MEMORY_DIR="${SESSIONS_DIR}"
  export MEMSEARCH_COLLECTION_NAME="${MEMSEARCH_COLLECTION}"
  mkdir -p "$MEMSEARCH_MEMORY_DIR" "$MEMSEARCH_STATE_DIR"
}

index_memory_top_level() {
  set -- "$NOTES_DIR" "$PROFILE_DIR"
  for path in "$SESSIONS_DIR"/*.md "$SESSIONS_DIR"/*.markdown; do
    [ -f "$path" ] && set -- "$@" "$path"
  done
  memsearch index "$@" --collection "$MEMSEARCH_COLLECTION" >/dev/null 2>&1
}
