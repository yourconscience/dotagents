# bin/memsearch

Portable Claude Code hook integration for memsearch. Reads all paths from `~/.agents/memsearch.conf` - no hardcoded directories.

## Setup

```bash
uv tool install memsearch
dotagents memsearch setup --vault ~/Workspace/knowledge
```

This creates the vault directory structure, initializes git, and writes `~/.agents/memsearch.conf`.

## hook.sh

Recursion-safe wrapper invoked by Claude Code hooks (SessionStart, Stop, SessionEnd). Sources `~/.agents/memsearch.conf` for all paths. Exits gracefully if memsearch is not configured.

Configure in Claude Code settings:
```json
{
  "hooks": {
    "SessionStart": [{"command": "bash ~/.agents/bin/memsearch/hook.sh session-start"}],
    "Stop": [{"command": "bash ~/.agents/bin/memsearch/hook.sh stop"}],
    "SessionEnd": [{"command": "bash ~/.agents/bin/memsearch/hook.sh session-end"}]
  }
}
```

## Config format

`~/.agents/memsearch.conf` (shell-sourceable):
```bash
MEMSEARCH_VAULT_DIR="$HOME/Workspace/knowledge"
MEMSEARCH_AI_DIR="$HOME/Workspace/knowledge/ai"
MEMSEARCH_NOTES_DIR="$HOME/Workspace/knowledge/notes"
MEMSEARCH_PROFILE_DIR="$HOME/Workspace/knowledge/profile"
MEMSEARCH_STATE_DIR="$HOME/.memsearch/state"
MEMSEARCH_COLLECTION="ai"
```
