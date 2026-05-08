# memory

Agent-agnostic session memory hooks and vault sync. The durable abstraction is
`memory/`; `memsearch` is the current indexing provider and remains an
implementation detail behind `~/.agents/memsearch.conf` and the `memsearch`
CLI.

## Setup

```bash
uv tool install memsearch
dotagents memsearch setup --vault ~/Workspace/knowledge
```

This creates the vault directory structure, initializes git, and writes `~/.agents/memsearch.conf`.

## Hooks

Hook entrypoints live under `~/.agents/memory/hooks/`:

- `session-start.sh`
- `stop.sh`
- `session-end.sh`
- `sync-memory-to-vault.sh`
- `sync-vault-to-memory.sh`

`session-end.sh` dispatches on hook payload shape for Claude Code, Droid, and
Hermes instead of keeping separate per-agent hook directories.

## Hermes

Configure in `~/.hermes/config.yaml`:
```yaml
hooks:
  on_session_finalize:
    - command: ~/.agents/memory/hooks/session-end.sh
      timeout: 30
    - command: ~/.agents/memory/hooks/sync-memory-to-vault.sh
      timeout: 15
  on_session_start:
    - command: ~/.agents/memory/hooks/sync-vault-to-memory.sh
      timeout: 15
```

First use requires normal Hermes shell-hook consent. `hermes hooks list` and `hermes hooks doctor` inspect the registration state.

Configure in Claude Code settings:
```json
{
  "hooks": {
    "SessionStart": [{"command": "bash ~/.agents/memory/hooks/session-start.sh"}],
    "Stop": [{"command": "bash ~/.agents/memory/hooks/stop.sh"}],
    "SessionEnd": [{"command": "bash ~/.agents/memory/hooks/session-end.sh"}]
  }
}
```

Configure in Droid settings:
```json
{
  "hooks": {
    "SessionEnd": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "/Users/conscience/Workspace/dotagents/memory/hooks/session-end.sh",
            "timeout": 60
          }
        ]
      }
    ]
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
