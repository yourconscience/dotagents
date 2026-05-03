#!/usr/bin/env bash
set -u

CONF="${MEMSEARCH_CONF:-$HOME/.agents/memsearch.conf}"
if [ -f "$CONF" ]; then
  # shellcheck source=/dev/null
  . "$CONF"
fi

TMP="$(mktemp)"
python3 "$HOME/.agents/bin/memsearch/sync.py" vault-to-memory >"$TMP" 2>&1
STATUS=$?
OUTPUT="$(cat "$TMP")"
rm -f "$TMP"

if [ -n "$OUTPUT" ]; then
  printf '%s\n' "$OUTPUT" >&2
fi

python3 - "$STATUS" "$OUTPUT" <<'PY'
import json
import sys
status = int(sys.argv[1])
message = sys.argv[2] if len(sys.argv) > 2 else ""
if len(message) > 500:
    message = message[:497] + "..."
print(json.dumps({
    "action": "continue",
    "message": message or "vault-to-memory sync completed",
    "exit_code": status,
}, ensure_ascii=False))
PY
exit "$STATUS"
