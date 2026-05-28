---
name: dotagents
description: Inspect and sync the repo-owned agent skill links for the dotagents repo across supported coding agents. Use when the user asks for dotagents status, dotagents sync, dotagents setup, or wants to reconcile ~/.agents with Amp/Claude/Codex/Hermes/Droid skill roots.
---

# dotagents

Use the root CLI tool. This skill is specific to the `~/Workspace/dotagents` repo and its managed symlink surface.

## Commands

From the repo root:

```bash
go install ./cmd/dotagents                  # install to Go bin dir
dotagents setup                             # first-time machine setup
dotagents status                            # show sync state
dotagents sync                              # sync skill symlinks
dotagents pull                              # git pull + sync (for cron)
dotagents cron --interval 30m               # install auto-pull crontab
dotagents cron --remove                     # remove crontab entry
dotagents mcp list                          # list canonical managed MCPs
dotagents mcp add local --command uvx --arg pkg@latest
dotagents mcp import claude-code local
dotagents mcp remove local
```

Ensure the Go install directory is on `PATH`. If `go env GOBIN` is non-empty, add that directory; otherwise add `$(go env GOPATH)/bin`.

Limit a run to specific agents:

```bash
dotagents sync --agents=claude-code,hermes
```

## Subcommands

### setup

First-time setup on a new machine. Does three things:
1. Creates `~/.agents` symlink pointing at the repo root.
2. Patches detected agent configs to load shared dotagents config where needed (Amp: sets `amp.skills.path` in `settings.json`; Hermes: adds `skills.external_dirs` in `config.yaml`).
3. Runs `sync` for managed skills, managed MCP entries, and supported hook entries.

Important Amp note: Amp should consume dotagents skills directly from `~/.agents/skills` via `amp.skills.path` in settings. Do not mirror repo skills into an Amp-specific directory, and do not commit project-local `.amp/` plugins or settings to this agent-agnostic repo. Let dotagents keep `~/.agents` canonical. Amp MCP entries are managed in the same settings file under `amp.mcpServers`. If an ignored local `.amp/settings.json` or `.amp/settings.jsonc` already exists, dotagents patches it because Amp gives workspace settings precedence; otherwise it uses `~/.config/amp/settings.json`.

If Amp suggests installing a plugin/skill instead of using the shared path, prefer pointing the install target back at dotagents so there is one source of truth:

```bash
amp skill add <source> --target ~/.agents/skills --overwrite
dotagents setup --agents=amp
```

Amp currently exposes this through `amp skill add`; keep the install target pointed at dotagents so there is one source of truth.

Important Hermes note: Hermes should consume dotagents skills primarily via `skills.external_dirs: ["~/.agents/skills"]`, not by mirroring repo skills into `~/.hermes/skills`. Hermes already ships a bundled categorized skill tree under `~/.hermes/skills`, so symlinking repo skills there can collide with bundled category directories. Example: a repo skill named `research` conflicts with Hermes' builtin `research/` category. Prefer Hermes-native bundled skills when an equivalent already exists there. Example: use Hermes' builtin `google-workspace` skill instead of trying to override it from dotagents. Treat `external_dirs` as the canonical Hermes integration path.

When a dotagents skill overlaps with a Hermes builtin skill, prefer the Hermes builtin and let the external dotagents copy exist only for other agents. Current example: the shared `skills/gws/` skill now has frontmatter name `google-workspace`, but on Hermes the builtin `google-workspace` skill should win and no extra sync action is needed.

### status

Reports sync state for each detected agent. Agents whose binary is not on PATH show "not detected" and are skipped. The report covers managed skills, native roles, MCP entries, and supported hook entries declared in `dotagents.yaml`.

### sync

Creates, updates, or removes skill symlinks in each detected agent's skill root for agents that use managed mirrors. Non-repo skills are reported as `external` and left untouched. Amp and Hermes are special-cased: `sync` verifies their config-driven shared-skill integrations and does not try to mirror repo skills into separate native skill roots.

For external skills (declared under `external_skills` in `dotagents.yaml`), `sync` clones or updates the remote git repos into `~/.agents/external/<repo-name>/`, discovers skills under the configured `skill_dir`, and symlinks them into agent skill roots alongside local skills.

For MCPs, `sync` patches only the managed server entries declared in `dotagents.yaml` and leaves unrelated MCP servers alone. Use `dotagents mcp add` or `dotagents mcp import` to update canonical `dotagents.yaml`, then run `dotagents sync` to distribute those MCPs to supported agents. If `--agents` is omitted, new/imported MCPs target all configured agents with MCP support (`amp`, `claude-code`, `codex`, `hermes`, `droid`, `pi`). `import` redacts native env values into `${KEY}` references (preserving existing `${SOME_VAR}` references); fill those values through environment variables or local native config as appropriate. `list` shows env key ***** only; it does not print env values. `remove` deletes only the canonical entry and does not remove native agent config entries.


For hooks, `sync` patches only managed hook entries declared in `dotagents.yaml` for agents with verified hook config support. It currently manages Claude Code hooks in `~/.claude/settings.json` and Hermes `SessionEnd` hooks in `~/.hermes/config.yaml`. It reports unsupported hook targets without failing. Hook approval is not automated; first-use approval and reapproval after script changes remain host-local user actions.

For repo-owned local MCP servers, prefer `mcp/<server-name>/` plus a canonical `dotagents.yaml` entry using `sh -lc 'exec ...'`, with local secrets/state outside git.

```bash
dotagents mcp list
dotagents mcp add local --command uvx --arg pkg@latest --env KEY=value
dotagents mcp import claude-code local --agents=amp,codex,hermes,droid
dotagents sync
dotagents mcp remove local
```

### pull

Runs `git pull --ff-only` in the repo root, then `sync`. Designed to be called from cron for auto-sync on remote machines.

### cron

Installs or removes a crontab entry that runs `pull` on a schedule. Default interval is 30m. Options: 5m, 15m, 30m, 1h, 6h, 12h, daily.

### promote

Promotes a Hermes skill to dotagents shared skills. Copies the skill, creates a branch, commits, pushes, and opens a PR.

```bash
dotagents promote SKILL_NAME          # by name (searches ~/.hermes/skills/)
dotagents promote <category>/<name>   # with category prefix
dotagents promote <name> --dry-run    # copy only, skip git/PR
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
dotagents dogfood
```

## What it manages

- Fixes `~/.agents` if it should point at the repo root and is missing or drifted.
- Configures Amp to load skills from `~/.agents/skills` through `~/.config/amp/settings.json`.
- Links Droid global instructions: `~/.factory/AGENTS.md -> ~/.agents/AGENTS.md` (real files conflict).
- Detects agents by checking if their binary is on PATH (`detect` field in config).
- Treats repo skills under `skills/` as the managed set for each detected agent.
- Treats repo hooks under `memory/hooks/` and skill hook entrypoints as managed only when declared in `dotagents.yaml`.
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

Hook scripts:

- `hooks/session-end.sh`: Appends a session digest to `$KNOWLEDGE_DIR/sessions/YYYY-MM-DD.md` and reindexes memsearch. Dispatches on hook payload shape for Claude Code, Droid, and Hermes.
- `hooks/sync.sh` -> `lib/sync.py`: Bidirectional sync between Hermes built-in memory files (`~/.hermes/memories/`) and the knowledge vault (`$KNOWLEDGE_DIR`). Three modes: `memory-to-vault`, `vault-to-memory`, `both`.
- `hooks/session-start.sh` and `hooks/stop.sh`: Claude Code memory hook shims when the memsearch Claude plugin is available.

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

The `knowledge-sync` tool source lives at `memory/tools/knowledge-sync/`. See its README for build and install instructions.

## MCP support

`dotagents` can also manage selected MCP server parity across agents without symlinking whole config files.

Current managed MCP set lives in root `dotagents.yaml` under `mcp_servers:`. Minimal examples in this repo:

- `linkedin` -> `uvx linkedin-scraper-mcp@latest`
- `telegram_readonly` -> repo-local Telegram MCP server

Status and sync rules:

- `dotagents status` reports `mcp managed`, `mcp missing`, and `mcp drifted` alongside skills.
- `dotagents sync` patches only the managed MCP entries for each supported agent.
- Unrelated MCP servers remain untouched.

Per-agent targets:

- Claude Code: `~/.claude/settings.json` -> `mcpServers.<name>`
- Amp: `~/.config/amp/settings.json` -> `amp.mcpServers.<name>`
- Codex: `~/.codex/config.toml` -> `[mcp_servers.<name>]`
- Hermes: `~/.hermes/config.yaml` -> `mcp_servers.<name>`
- Factory Droid: `~/.factory/mcp.json` -> `mcpServers.<name>`
- Pi: `~/.omp/agent/mcp.json` -> `mcpServers.<name>`

Do not symlink whole agent config files. Use targeted patches only. Droid MCP config is patched in-place at `~/.factory/mcp.json`; it is not symlinked.

## Config

Default targets live in:

```bash
dotagents.yaml
```

Use `--agents` only as a one-run override.
