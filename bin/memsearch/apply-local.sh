#!/usr/bin/env bash
# Re-apply local patches to ~/.agents/memsearch after a `git pull`.
# Location: ~/.agents/bin/memsearch/apply-local.sh
#
# The patches make two env vars authoritative so our wrapper hook can force
# memsearch to use a global memory dir and a fixed Milvus collection,
# regardless of the current project directory.
#
# Idempotent: running twice is a no-op. Safe to call from any post-merge flow.

set -euo pipefail

MS_DIR="$HOME/Public/memsearch"
COMMON_SH="$MS_DIR/plugins/claude-code/hooks/common.sh"
DERIVE_SH="$MS_DIR/plugins/claude-code/scripts/derive-collection.sh"
SESSION_START_SH="$MS_DIR/plugins/claude-code/hooks/session-start.sh"
STOP_SH="$MS_DIR/plugins/claude-code/hooks/stop.sh"

for f in "$COMMON_SH" "$DERIVE_SH" "$SESSION_START_SH" "$STOP_SH"; do
  if [ ! -f "$f" ]; then
    echo "memsearch file missing: $f" >&2
    exit 1
  fi
done

# Patch 1: common.sh - make MEMSEARCH_DIR / MEMORY_DIR env-overridable
if ! grep -q 'MEMSEARCH_MEMORY_DIR' "$COMMON_SH"; then
  python3 - <<PY
from pathlib import Path
p = Path("$COMMON_SH")
src = p.read_text()
old = 'MEMSEARCH_DIR="\$_PROJECT_DIR/.memsearch"\nMEMORY_DIR="\$MEMSEARCH_DIR/memory"'
new = 'MEMSEARCH_DIR="\${MEMSEARCH_STATE_DIR:-\$_PROJECT_DIR/.memsearch}"\nMEMORY_DIR="\${MEMSEARCH_MEMORY_DIR:-\$MEMSEARCH_DIR/memory}"'
if old not in src:
    raise SystemExit("common.sh: upstream pattern missing - check for upstream changes")
p.write_text(src.replace(old, new))
print("patched common.sh")
PY
else
  echo "common.sh already patched"
fi

# Patch 2: derive-collection.sh - honor MEMSEARCH_COLLECTION_NAME env override
if ! grep -q 'MEMSEARCH_COLLECTION_NAME' "$DERIVE_SH"; then
  python3 - <<PY
from pathlib import Path
p = Path("$DERIVE_SH")
src = p.read_text()
anchor = 'set -euo pipefail\n\nPROJECT_DIR="\${1:-\$(pwd)}"'
inject = '''set -euo pipefail

# Local override: allow forcing a fixed collection name via env var.
# Used by our hook wrapper to share one collection across all projects.
if [ -n "\${MEMSEARCH_COLLECTION_NAME:-}" ]; then
  echo "\$MEMSEARCH_COLLECTION_NAME"
  exit 0
fi

PROJECT_DIR="\${1:-\$(pwd)}"'''
if anchor not in src:
    raise SystemExit("derive-collection.sh: upstream pattern missing - check for upstream changes")
p.write_text(src.replace(anchor, inject))
print("patched derive-collection.sh")
PY
else
  echo "derive-collection.sh already patched"
fi

# Patch 3: session-start.sh - drop eager "## Session HH:MM" heading write
if ! grep -q 'Local patch: do NOT write session heading here' "$SESSION_START_SH"; then
  python3 - <<PY
from pathlib import Path
p = Path("$SESSION_START_SH")
src = p.read_text()
old = '''# Write session heading to today's memory file
ensure_memory_dir
TODAY=\$(date +%Y-%m-%d)
NOW=\$(date +%H:%M)
MEMORY_FILE="\$MEMORY_DIR/\$TODAY.md"
if [ ! -f "\$MEMORY_FILE" ] || ! grep -qF "## Session \$NOW" "\$MEMORY_FILE"; then
  echo -e "\\n## Session \$NOW\\n" >> "\$MEMORY_FILE"
fi'''
new = '''# Local patch: do NOT write session heading here.
# stop.sh now writes the session heading lazily (only when there's real turn
# content to record). This avoids 60%+ empty "## Session HH:MM" blocks created
# by nested claude -p invocations that never have a real user turn.
ensure_memory_dir
TODAY=\$(date +%Y-%m-%d)
MEMORY_FILE="\$MEMORY_DIR/\$TODAY.md"'''
if old not in src:
    raise SystemExit("session-start.sh: upstream pattern missing - check for upstream changes")
p.write_text(src.replace(old, new))
print("patched session-start.sh")
PY
else
  echo "session-start.sh already patched"
fi

# Patch 4: stop.sh - new heading format (## HH:MM claude-code) + italic anchor
# NOTE: we do NOT add --bare to the child claude call because Claude Max uses
# OAuth and --bare refuses to read OAuth/keychain auth. Recursion is blocked
# instead by the MEMSEARCH_SKIP_CLAUDE_HOOKS env var guard in memsearch-hook.sh.
if ! grep -q 'Local patch: write a top-level H2 heading' "$STOP_SH"; then
  python3 - <<PY
from pathlib import Path
p = Path("$STOP_SH")
src = p.read_text()
old = '''# Append as a sub-heading under the session heading written by SessionStart
# Include HTML comment anchor for progressive disclosure (L3 transcript lookup)
{
  echo "### \$NOW"
  if [ -n "\$SESSION_ID" ]; then
    echo "<!-- session:\${SESSION_ID} turn:\${LAST_USER_TURN_UUID} transcript:\${TRANSCRIPT_PATH} -->"
  fi
  echo "\$SUMMARY"
  echo ""
} >> "\$MEMORY_FILE"'''
new = '''# Local patch: write a top-level H2 heading per turn with an italic anchor
# (Obsidian-friendly replacement for the old HTML-comment anchor). Per-turn
# entries are consolidated into one session-level digest by our SessionEnd
# wrapper, so duplicate headings for the same session are fine here.
{
  echo ""
  echo "## \$NOW claude-code"
  if [ -n "\$SESSION_ID" ]; then
    echo "*session \\`\${SESSION_ID}\\` - [raw transcript](\${TRANSCRIPT_PATH})*"
    echo ""
  fi
  echo "\$SUMMARY"
  echo ""
} >> "\$MEMORY_FILE"'''
if old not in src:
    raise SystemExit("stop.sh: upstream pattern missing - check for upstream changes")
p.write_text(src.replace(old, new))
print("patched stop.sh")
PY
else
  echo "stop.sh already patched"
fi

echo "all patches applied"
