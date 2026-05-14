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
go run ./skills/dotagents/tools/dotagents mcp list             # list canonical managed MCPs
go run ./skills/dotagents/tools/dotagents mcp add local --command uvx --arg pkg@latest
go run ./skills/dotagents/tools/dotagents mcp import claude-code local
go run ./skills/dotagents/tools/dotagents mcp remove local
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

For MCPs, `sync` patches only the managed server entries declared in `dotagents.yaml` and leaves unrelated MCP servers alone. Use `dotagents mcp add` or `dotagents mcp import` to update canonical `skills/dotagents/dotagents.yaml`, then run `dotagents sync` to distribute those MCPs to supported agents. If `--agents` is omitted, new/imported MCPs target all configured agents with MCP support (`claude-code`, `codex`, `hermes`, `droid`). `import` redacts native env values into `${KEY}` references (preserving existing `${SOME_VAR}` references); fill those values through environment variables or local native config as appropriate. `list` shows env key ***** only; it does not print env values. `remove` deletes only the canonical entry and does not remove native agent config entries.

```bash
go run ./skills/dotagents/tools/dotagents mcp list
go run ./skills/dotagents/tools/dotagents mcp add local --command uvx --arg pkg@latest --env KEY=value
go run ./skills/dotagents/tools/dotagents mcp import claude-code local --agents=codex,hermes,droid
go run ./skills/dotagents/tools/dotagents sync
go run ./skills/dotagents/tools/dotagents mcp remove local
```

### pull

Runs `git pull --ff-only` in the repo root, then `sync`. Designed to be called from cron for auto-sync on remote machines.

### cron

Installs or removes a crontab entry that runs `pull` on a schedule. Default interval is 30m. Options: 5m, 15m, 30m, 1h, 6h, 12h, daily.

### promote

Promotes a Hermes skill to dotagents shared skills. Copies the skill, creates a branch, commits, pushes, and opens a PR.

```bash
go run ./skills/dotagents/tools/dotagents promote SKILL_NAME          # by name (searches ~/.hermes/skills/)
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
- Links Droid global instructions: `~/.factory/AGENTS.md -> ~/.agents/AGENTS.md` (real files conflict).
- Detects agents by checking if their binary is on PATH (`detect` field in config).
- Treats repo skills under `skills/` as the managed set for each detected agent.
- Renders canonical repo roles under `agents/*.yaml` to each detected agent's native `agent_root` format where supported:
  - Claude Code: `~/.claude/agents/<name>.md`
  - Codex: `~/.codex/agents/<name>.toml`
  - Factory Droid: `~/.factory/droids/<name>.md`
- Stops on conflicts when a native agent file with the same name already exists and was not generated by dotagents.
- Reports non-repo skills already present in agent skill roots as `external` and leaves them untouched.
- Stops on conflicts when a managed skill path exists as a real file or directory instead of a symlink.

## Root instruction shims

Keep `AGENTS.md` canonical. `CLAUDE.md` points agents to it. Droid uses the same canonical file through `~/.factory/AGENTS.md -> ~/.agents/AGENTS.md`; deleting `~/.factory/AGENTS.md` does not make Droid discover `~/.agents/AGENTS.md`.

```bash
readlink ~/.agents
cmp -s CLAUDE.md ~/.agents/CLAUDE.md && echo "CLAUDE.md visible via ~/.agents"
```

## Memory sync

The dotagents repo also owns the agent memory ↔ knowledge vault sync pipeline at `~/.agents/memory/`. This is separate from skill/MCP sync but lives in the same repo.

Two scripts:

- `hooks/session-end.sh`: Appends a session digest to `~/Workspace/knowledge/ai/YYYY-MM-DD.md` and reindexes memsearch. Dispatches on hook payload shape for Claude Code, Droid, and Hermes.
- `hooks/sync.sh` → `lib/sync.py`: Bidirectional sync between Hermes built-in memory files (`~/.hermes/memories/`) and the knowledge vault (`~/Workspace/knowledge/`). Three modes: `memory-to-vault`, `vault-to-memory`, `both`.

The typical Hermes hook pipeline:

```yaml
on_session_finalize:
  - command: ~/.agents/memory/hooks/session-end.sh
    timeout: 30
  - command: ~/.agents/memory/hooks/sync.sh
    args:
    - memory-to-vault
    timeout: 15
```

Hook approval: first-use consent requires a TTY prompt. Scripts modified after approval require revoke + re-approve via `hermes hooks revoke <command>` then approve at the next session-end TTY prompt. Cannot be automated from CLI.

Hook health pitfall: `hermes hooks doctor` requires hook stdout to be valid JSON. Plain-text sync wrapper output can be allowlisted/executable but still fail doctor with `stdout was not valid JSON`; fix the wrapper contract only when ready to re-approve the modified script in a TTY.

For detailed pitfalls (char limits, silent sync failures, hook debugging, JSON stdout requirements, and re-approval after wrapper changes), see `references/memory-sync.md`.

If you find a standalone `knowledge-sync` helper or source tree, treat it as part of the dotagents memory subsystem. Keep its source under `memory/tools/knowledge-sync/` and keep launchd pointing at the stable compiled binary under `~/.local/bin`. Before moving it, inspect LaunchAgents, processes, shell/config references, binary build metadata, logs, and current vault git health. See `references/knowledge-sync-tool.md` for the checklist and migration pattern.

## MCP support

`dotagents` can also manage selected MCP server parity across agents without symlinking whole config files.

Current managed MCP set lives in `skills/dotagents/dotagents.yaml` under `mcp_servers:`. Minimal example in this repo:

- `linkedin` -> `uvx linkedin-scraper-mcp@latest`
- managed for `claude-code`, `codex`, `hermes`, and `droid`

Status and sync rules:

- `dotagents status` reports `mcp managed`, `mcp missing`, and `mcp drifted` alongside skills.
- `dotagents sync` patches only the managed MCP entries for each supported agent.
- Unrelated MCP servers remain untouched.

Per-agent targets:

- Claude Code: `~/.claude/settings.json` -> `mcpServers.<name>`
- Codex: `~/.codex/config.toml` -> `[mcp_servers.<name>]`
- Hermes: `~/.hermes/config.yaml` -> `mcp_servers.<name>`
- Factory Droid: `~/.factory/mcp.json` -> `mcpServers.<name>`

Do not symlink whole agent config files. Use targeted patches only. Droid MCP config is patched in-place at `~/.factory/mcp.json`; it is not symlinked.

## Config

Default targets live in:

```bash
skills/dotagents/dotagents.yaml
```

Use `--agents` only as a one-run override.
