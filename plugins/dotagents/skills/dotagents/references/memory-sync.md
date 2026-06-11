# Hermes Memory Sync — pitfalls and debugging

## Char limits

Hermes built-in memory files have hard limits:
- `~/.hermes/memories/MEMORY.md`: 2,200 chars
- `~/.hermes/memories/USER.md`: 1,375 chars

The `memory` tool enforces these on writes. `sync.py` also respects them.

## Silent vault→memory sync failure

**Symptom**: `sync.py vault-to-memory` reports `added 0 facts` but the vault profile has facts not in Hermes memory.

**Root cause**: USER.md is near capacity. The script detects new facts correctly but can't fit them within the limit.

**Debug**: Run this extraction to see what's missing and why:

```python
from pathlib import Path
import re

def normalize(entry):
    return re.sub(r"\s+", " ", entry.lower().strip())

# Parse Hermes USER.md
hermes_user = Path.home() / ".hermes" / "memories" / "USER.md"
text = hermes_user.read_text().strip()
hermes_entries = [e.strip() for e in text.split("§") if e.strip()]
hermes_norm = {normalize(e) for e in hermes_entries}

# Parse vault profile
profile = Path.home() / "Workspace" / "knowledge" / "profile" / "USER.md"
profile_facts = []
for line in profile.read_text().splitlines():
    stripped = line.strip()
    if stripped.startswith("- "):
        fact = stripped[2:].strip()
        if fact and len(fact) > 15:
            profile_facts.append(fact)

new_facts = [f for f in profile_facts if normalize(f) not in hermes_norm]
print(f"New facts: {len(new_facts)}, available chars: {1375 - sum(len(e)+1 for e in hermes_entries)}")
for f in new_facts:
    print(f"  [{len(f)} chars] {f}")
```

**Fix**: Consolidate redundant entries in Hermes memory using the `memory` tool. Merge related facts (e.g., all project transition details into one entry). Remove one-time facts (e.g., "backup complete"). Then re-run `sync.py memory-to-vault` and force-push the corrected vault branch. Verify with `hermes hooks doctor`.

## Hook approval loop

- Hook scripts at `~/.agents/memory/hooks/` require first-use TTY approval.
- `hermes hooks doctor` reports `script modified since approval` when mtime drifts — requires `hermes hooks revoke <command>`, then re-approve at next session-end TTY prompt.
- `--accept-hooks` flag applies only to the current `hermes hooks test` invocation, does NOT persist approval.
- Hook approval is stored in `~/.hermes/shell-hooks-allowlist.json`.
- The allowlist matches the **exact command string** from `config.yaml`.

## sync.py architecture

`~/.agents/memory/lib/sync.py` does bidirectional sync:

| Direction | Source → Target | What moves |
|-----------|----------------|------------|
| `memory-to-vault` | Hermes MEMORY.md → `ai/knowledge.md` | Memory facts as bullet list |
| | Hermes USER.md → `profile/USER.md` | User facts under `## Hermes Memory Sync` section |
| `vault-to-memory` | `profile/USER.md` → Hermes USER.md | Bullet points from main profile sections |

After any change, reindexes memsearch via `memsearch index`.

The `profile/USER.md` `## Hermes Memory Sync` section should be treated as a managed mirror of Hermes `USER.md`, not an incremental append log. Rewriting the whole section makes `memory-to-vault` idempotent and prevents oscillation where each run alternates which subset of consolidated facts appears.

The `vault-to-memory` direction only imports bullets starting with `- ` that are >15 chars and not already present in Hermes memory. Dedup must handle consolidated Hermes entries: exact normalized equality is not enough. Use rendered-size accounting for `\n§\n` separators when enforcing the 1,375 char USER.md limit, and skip profile bullets that are substring/token-overlap duplicates of existing consolidated entries.

## Hook verification

Test hooks with synthetic payload:
```bash
hermes hooks test on_session_finalize
```

Check hook health:
```bash
hermes hooks doctor
hermes hooks list
```

View allowlist:
```bash
cat ~/.hermes/shell-hooks-allowlist.json
```

## JSON stdout requirement

`hermes hooks doctor` expects hook stdout to be valid JSON. `finalize.py` already prints JSON, but plain `sync.py` output such as `memory→vault: ...` or `vault→memory: ...` is treated as a doctor failure even with exit code 0.

Important distinction:
- Live hook execution still spawns the script; invalid JSON means Hermes ignores the hook response payload, not that the side effect necessarily failed.
- Doctor remains red and should not be ignored long-term.
- Fixing wrapper stdout after approval changes the script mtime/hash, so Hermes will require hook re-approval. From Telegram/mobile this is a bad time to patch unless `hooks_auto_accept` is intentionally enabled or the user can approve from a TTY.

Preferred repair when a TTY is available:
1. Make the sync wrapper capture `sync.py` logs to stderr or a log file and print one JSON object to stdout, e.g. `{"action":"continue","message":"memory sync complete"}`.
2. Run `hermes hooks revoke <command>` if doctor reports modified-since-approval.
3. Trigger or test hooks from a TTY and approve, or update the allowlist deliberately only when the command/path is unchanged and the user has explicitly asked for that local repair.
4. Verify `hermes hooks doctor` is fully green.

Known-good wrapper shape:
- `set -u`, not `set -eu`, so the wrapper can serialize a non-zero child exit into JSON before exiting with that status.
- Capture `python3 ~/.agents/memory/lib/sync.py <direction>` stdout+stderr to a temp file.
- Echo captured logs to stderr for debugging.
- Print exactly one JSON object to stdout: `{"action":"continue","message":"...","exit_code":0}`.
- Exit with the child status.
