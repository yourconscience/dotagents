#!/usr/bin/env bash
set -eu

. "$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)/common.sh"

if [ "${MEMSEARCH_SKIP_CLAUDE_HOOKS:-}" = "1" ]; then
  echo '{}'
  exit 0
fi

load_memory_config
prepare_memory_index_env

payload="$(mktemp)"
trap 'rm -f "$payload"' EXIT
cat >"$payload"

kind="$(python3 - "$payload" <<'PY'
import json
import sys
from pathlib import Path

try:
    data = json.load(open(sys.argv[1]))
except Exception:
    print("unknown")
    raise SystemExit

transcript = str(data.get("transcript_path") or "")
if transcript and "/.factory/sessions/" in transcript and Path(transcript).suffix == ".jsonl":
    print("factory-jsonl")
elif data.get("session_id") and not transcript:
    print("hermes-json")
else:
    print("claude-plugin")
PY
)"

case "$kind" in
  factory-jsonl)
    exec python3 "$MEMORY_DIR/lib/factory_digest.py" <"$payload"
    ;;
  hermes-json)
    exec python3 "$MEMORY_DIR/lib/hermes_digest.py" <"$payload"
    ;;
  *)
    plugin_dir="$(resolve_claude_memory_plugin || true)"
    if [ -n "$plugin_dir" ]; then
      bash "$plugin_dir/hooks/session-end.sh" <"$payload" >/dev/null 2>&1 || true
      index_memory_top_level || true
    fi
    printf '{"continue":true,"suppressOutput":true}\n'
    ;;
esac
