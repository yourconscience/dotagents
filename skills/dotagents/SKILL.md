---
name: dotagents
description: Inspect and sync the repo-owned agent skill links for the dotagents repo across supported coding agents. Use when the user asks for dotagents status, dotagents sync, dotagents setup, or wants to reconcile ~/.agents with Claude/Codex/Hermes/OpenClaw skill roots.
---

# dotagents

Use the skill-local CLI tool. This skill is specific to the `~/Workspace/dotagents` repo and its managed symlink surface.

## Commands

From the repo root:

```bash
go run ./skills/dotagents/tools/dotagents setup                # first-time machine setup
go run ./skills/dotagents/tools/dotagents status               # show sync state
go run ./skills/dotagents/tools/dotagents sync                 # sync skill symlinks
go run ./skills/dotagents/tools/dotagents pull                 # git pull + sync (for cron)
go run ./skills/dotagents/tools/dotagents cron --interval 30m  # install auto-pull crontab
go run ./skills/dotagents/tools/dotagents cron --remove        # remove crontab entry
```

Limit a run to specific agents:

```bash
go run ./skills/dotagents/tools/dotagents sync --agents=claude-code,hermes
```

## Subcommands

### setup

First-time setup on a new machine. Does three things:
1. Creates `~/.agents` symlink pointing at the repo root.
2. Patches detected agent configs to load shared dotagents config where needed (Hermes: adds `skills.external_dirs` in `config.yaml`).
3. Runs `sync` for managed skills and managed MCP entries.

Important Hermes note: Hermes should consume dotagents skills primarily via `skills.external_dirs: ["~/.agents/skills"]`, not by mirroring repo skills into `~/.hermes/skills`. Hermes already ships a bundled categorized skill tree under `~/.hermes/skills`, so symlinking repo skills there can collide with bundled category directories. Example: a repo skill named `research` conflicts with Hermes' builtin `research/` category. Prefer Hermes-native bundled skills when an equivalent already exists there. Example: use Hermes' builtin `google-workspace` skill instead of trying to override it from dotagents. Treat `external_dirs` as the canonical Hermes integration path.

When a dotagents skill overlaps with a Hermes builtin skill, prefer the Hermes builtin and let the external dotagents copy exist only for other agents. Current example: the shared `skills/gws/` skill now has frontmatter name `google-workspace`, but on Hermes the builtin `google-workspace` skill should win and no extra sync action is needed.

### status

Reports sync state for each detected agent. Agents whose binary is not on PATH show "not detected" and are skipped. The report covers both managed skills and any managed MCP entries declared in `dotagents.yaml`.

### sync

Creates, updates, or removes skill symlinks in each detected agent's skill root for agents that use managed mirrors. Non-repo skills are reported as `external` and left untouched. Hermes is special-cased: `sync` verifies the `skills.external_dirs` integration and does not try to mirror repo skills into `~/.hermes/skills`.

For MCPs, `sync` patches only the managed server entries declared in `dotagents.yaml` and leaves unrelated MCP servers alone.

### pull

Runs `git pull --ff-only` in the repo root, then `sync`. Designed to be called from cron for auto-sync on remote machines.

### cron

Installs or removes a crontab entry that runs `pull` on a schedule. Default interval is 30m. Options: 5m, 15m, 30m, 1h, 6h, 12h, daily.

### promote

Promotes a Hermes skill to dotagents shared skills. Copies the skill, creates a branch, commits, pushes, and opens a PR.

```bash
go run ./skills/dotagents/tools/dotagents promote <skill-name>        # by name (searches ~/.hermes/skills/)
go run ./skills/dotagents/tools/dotagents promote <category>/<name>   # with category prefix
go run ./skills/dotagents/tools/dotagents promote <name> --dry-run    # copy only, skip git/PR
```

The skill is searched in `~/.hermes/skills/` by name (including inside category subdirectories). The promote command:
1. Copies the skill to `skills/<name>/`
2. Creates branch `promote/<name>`
3. Commits as "add <name> skill"
4. Pushes and creates a PR via `gh`

Requires `gh` CLI authenticated with push access to the repo.

For the full promotion workflow (evaluation, pr-triage, merge), use the `skill-promote` Hermes skill.

### dogfood

End-to-end self-test: runs sync, then status, then doctor. Fails on any drift, conflict, or doctor warning. Use after making changes to skills or config to verify the full pipeline.

```bash
go run ./skills/dotagents/tools/dotagents dogfood
```

## What it manages

- Fixes `~/.agents` if it should point at the repo root and is missing or drifted.
- Detects agents by checking if their binary is on PATH (`detect` field in config).
- Treats repo skills under `skills/` as the managed set for each detected agent.
- Renders canonical repo roles under `agents/*.yaml` to each detected agent's native `agent_root` format where supported:
  - Claude Code: `~/.claude/agents/<name>.md`
  - Codex: `~/.codex/agents/<name>.toml`
- Stops on conflicts when a native agent file with the same name already exists and was not generated by dotagents.
- Reports non-repo skills already present in agent skill roots as `external` and leaves them untouched.
- Stops on conflicts when a managed skill path exists as a real file or directory instead of a symlink.

## Root instruction shims and local agent state

For cross-agent repo instructions, keep `AGENTS.md` canonical and add a small root `CLAUDE.md` shim that points agents to `AGENTS.md`. Do not expect `dotagents sync` to separately install or mirror root files: root files are available to tools through the `~/.agents -> repo` symlink. Verify with:

```bash
readlink ~/.agents
cmp -s CLAUDE.md ~/.agents/CLAUDE.md && echo "CLAUDE.md visible via ~/.agents"
```

When cleaning committed Claude Code runtime artifacts, ignore `.claude/` wholesale unless there is a deliberate shared Claude project config to track. `.claude/worktrees/*` can be committed accidentally as gitlinks, and `.claude/settings.local.json` is local runtime state. If shared Claude config is needed later, use explicit negation patterns in `.gitignore` rather than narrowly ignoring only known generated files.

## Memory sync

The dotagents repo also owns the Hermes memory ↔ knowledge vault sync pipeline at `~/.agents/bin/memsearch/`. This is separate from skill/MCP sync but lives in the same repo.

Runtime scripts:

- `finalize.sh` → `finalize.py`: Appends a session digest to `~/Workspace/knowledge/ai/YYYY-MM-DD.md` and reindexes memsearch. Fires on `on_session_finalize`.
- `sync.py`: Bidirectional sync implementation between Hermes built-in memory files (`~/.hermes/memories/`) and the knowledge vault (`~/Workspace/knowledge/`). Modes: `memory-to-vault`, `vault-to-memory`, `both`.
- `sync-memory-to-vault.sh`: JSON-stdout wrapper for `sync.py memory-to-vault`. Fires on `on_session_finalize`.
- `sync-vault-to-memory.sh`: JSON-stdout wrapper for `sync.py vault-to-memory`. Fires on `on_session_start`.
- `sync.sh`: helper/backcompat entrypoint; do not prefer it for Hermes hooks when separate JSON wrappers are available.

The current Hermes hook pipeline should use separate wrappers with no args:

```yaml
on_session_finalize:
  - command: ~/.agents/bin/memsearch/finalize.sh
    timeout: 30
  - command: ~/.agents/bin/memsearch/sync-memory-to-vault.sh
    timeout: 15
on_session_start:
  - command: ~/.agents/bin/memsearch/sync-vault-to-memory.sh
    timeout: 15
```

Hook approval: first-use consent normally requires a TTY prompt. Scripts modified after approval require either revoke + re-approve via `hermes hooks revoke <command>` from a TTY, or a deliberate allowlist approval metadata update only when the command/path is unchanged and the user explicitly asked for that local repair.

Hook health pitfall: `hermes hooks doctor` requires hook stdout to be valid JSON. Plain-text sync wrapper output can be allowlisted/executable but still fail doctor with `stdout was not valid JSON`; the wrapper contract is: human logs to stderr, exactly one JSON object to stdout, and exit with the child status.

For detailed pitfalls (char limits, silent sync failures, hook debugging, JSON stdout requirements, and re-approval after wrapper changes), see `references/memory-sync.md`.

## MCP support

`dotagents` can also manage selected MCP server parity across agents without symlinking whole config files.

Current managed MCP set lives in `skills/dotagents/dotagents.yaml` under `mcp_servers:`. Minimal example in this repo:

- `linkedin` -> `uvx linkedin-scraper-mcp@latest`
- managed for `claude-code`, `codex`, and `hermes`

Status and sync rules:

- `dotagents status` reports `mcp managed`, `mcp missing`, and `mcp drifted` alongside skills.
- `dotagents sync` patches only the managed MCP entries for each supported agent.
- Unrelated MCP servers remain untouched.

Per-agent targets:

- Claude Code: `~/.claude/settings.json` -> `mcpServers.<name>`
- Codex: `~/.codex/config.toml` -> `[mcp_servers.<name>]`
- Hermes: `~/.hermes/config.yaml` -> `mcp_servers.<name>`

Do not symlink whole agent config files. Use targeted patches only.

## Config

Default targets live in:

```bash
skills/dotagents/dotagents.yaml
```

Use `--agents` only as a one-run override.
