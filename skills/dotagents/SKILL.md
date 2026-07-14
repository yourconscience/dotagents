---
name: dotagents
description: Inspect and sync the repo-owned agent surfaces across Claude Code, Codex, Hermes, Droid, Pi, and OMP. Use for dotagents setup, status, sync, doctor, skill, MCP, or cron workflows; Amp/OpenClaw/OpenCode-style harnesses are compatibility-only unless explicitly configured.
---

# dotagents

Use the root CLI from the repository checkout.

## Commands

From the repo root, use the canonical public grammar:

```bash
go install ./cmd/dotagents
dotagents setup [--delivery plugin|sync] [--agents ...]
dotagents status [--agents ...]
dotagents sync [--pull] [--agents ...]
dotagents doctor [--e2e] [--agents ...]
dotagents skill new <name> [--description ...]
dotagents skill update [name ...]
dotagents skill promote <name-or-path> [--dry-run]
dotagents mcp <list|add|import|remove> [options]
dotagents cron [--interval 30m|--deps|--remove]
```

Run `dotagents help --all` for maintenance commands and explicitly labeled compatibility aliases. Do not use those aliases in new commands or documentation.

Ensure the Go install directory is on `PATH`. If `go env GOBIN` is non-empty, add that directory; otherwise add `$(go env GOPATH)/bin`.

Limit supported commands to specific agents with `--agents`, for example:

```bash
dotagents sync --agents=claude-code,hermes
```

## Native plugin installation

Claude Code uses in-session marketplace commands:

```text
/plugin marketplace add yourconscience/dotagents
/plugin install dotagents@yourconscience
```

Codex uses its CLI marketplace commands:

```bash
codex plugin marketplace add yourconscience/dotagents
codex plugin add dotagents@yourconscience
```

Claude Code plugin delivery is selected for managed machines with `dotagents setup --delivery plugin`; switch back with `dotagents setup --delivery sync`. The Codex marketplace package is zero-copy: `.agents/plugins/marketplace.json` selects `./skills`, and `skills/.codex-plugin/plugin.json` exposes that canonical tree with `"skills": "./"`. No rendered mirror or plugin render step is required.

## Subcommands

### setup

First-time setup on a new machine. Does three things:
1. Creates `~/.agents` symlink pointing at the repo root.
2. Patches detected primary agent configs to load shared dotagents config where needed (Hermes: adds `skills.external_dirs` in `config.yaml`).
3. Runs `sync` for managed skills, managed MCP entries, and supported hook entries.

Use `--delivery plugin` or `--delivery sync` to select Claude Code delivery without creating a dual-delivery state.

Amp compatibility note: Amp support remains in the CLI for explicit local configs, migration, and cleanup, but Amp is no longer a canonical `dotagents.yaml` target. Do not add Amp to managed plugin/MCP/skill targets unless the user explicitly asks for that one-off integration.

Pi and OMP are distinct targets:

- Pi detects the `pi` binary and supports skills only at the configured `~/.pi/agent/skills` root. It has no managed MCP, hooks, or native role surface.
- OMP detects the `omp` binary and supports skills at `~/.omp/agent/skills`, native roles at `~/.omp/agent/agents`, and managed MCP entries in `~/.omp/agent/mcp.json`. It has no managed hook surface.

An OMP installation may also expose a `pi` alias. Because detection checks binary names, both targets can then appear detected; keep their roots and capabilities distinct rather than treating the alias as shared identity.

Important Hermes note: Hermes should consume dotagents skills primarily via `skills.external_dirs: ["~/.agents/skills"]`, not by mirroring repo skills into `~/.hermes/skills`. Hermes already ships a bundled categorized skill tree under `~/.hermes/skills`, so symlinking repo skills there can collide with bundled category directories. Example: a repo skill named `research` conflicts with Hermes' builtin `research/` category. Prefer Hermes-native bundled skills when an equivalent already exists there. Example: use Hermes' builtin `google-workspace` skill instead of trying to override it from dotagents. Treat `external_dirs` as the canonical Hermes integration path.

When a dotagents skill overlaps with a Hermes builtin skill, prefer the Hermes builtin and let the external dotagents copy exist only for other agents. Current example: the shared `skills/gws/` skill now has frontmatter name `google-workspace`, but on Hermes the builtin `google-workspace` skill should win and no extra sync action is needed.

### status

Reports sync state for each detected agent. Agents whose binary is not on PATH show "not detected" and are skipped. The report covers managed skills, native roles, MCP entries, and supported hook entries declared in `dotagents.yaml`.

### sync

Creates, updates, or removes skill symlinks in each detected primary agent's skill root for agents that use managed mirrors. Non-repo skills are reported as `external` and left untouched. Hermes is special-cased: `sync` verifies its config-driven shared-skill integration and does not mirror repo skills into `~/.hermes/skills`.

For external Git skills declared under `external_skills`, `sync` reuses one checkout per repository under `~/.agents/external/<repo-name>/` and records the exact commit in `dotagents.lock`. Entries may keep the backward-compatible direct-cache delivery (`skill_dir` plus optional `skills`) or use `skill_dirs` with `materialize: true` to copy selected trees into canonical `skills/<basename>`. Materialized copies are the only delivered paths for those entries; `sync` repairs drift from the lock and `skill update` advances the pin before refreshing copies. `doctor` fails on drift or direct-cache delivery of a materialized skill and scans the external source for risky patterns.

Enabled native-plugin skill surfaces are discovered from installed plugin directories and projected to configured skill-capable harnesses. They remain owned by the plugin installation and are not represented as pinned or audited external Git repositories in `dotagents.lock`.

For MCPs, `sync` patches only the managed server entries declared in `dotagents.yaml` and leaves unrelated MCP servers alone. Use `dotagents mcp add` or `dotagents mcp import` to update canonical `dotagents.yaml`, then run `dotagents sync` to distribute those MCPs to supported agents. If `--agents` is omitted, new/imported MCPs target the configured primary agents with MCP support (`claude-code`, `codex`, `hermes`, `droid`, `omp`). `import` redacts native env values into `${KEY}` references (preserving existing `${SOME_VAR}` references); fill those values through environment variables or local native config as appropriate. `list` shows env key ***** only; it does not print env values. `remove` deletes only the canonical entry and does not remove native agent config entries.


For hooks, `sync` patches only managed hook entries declared in `dotagents.yaml` for agents with verified hook config support. It manages Claude Code hooks in `~/.claude/settings.json`, Codex hooks in `~/.codex/hooks.json` plus `[features].hooks = true`, Factory Droid hooks in `~/.factory/settings.json`, and Hermes hooks in `~/.hermes/config.yaml`. It reports unsupported hook targets without failing. Hook approval is not automated; first-use approval and reapproval after script changes remain host-local user actions.

For repo-owned local MCP servers, prefer `mcp/<server-name>/` plus a canonical `dotagents.yaml` entry using `sh -lc 'exec ...'`, with local secrets/state outside git.

```bash
dotagents mcp list
dotagents mcp add local --command uvx --arg pkg@latest --env KEY=value
dotagents mcp import claude-code local --agents=codex,hermes,droid,omp
dotagents sync
dotagents mcp remove local
```

### sync --pull

Runs `git pull --ff-only` in the repo root before reconciling all managed surfaces.

### cron

Installs or removes a crontab entry that runs `pull` on a schedule. Default interval is 30m. Options: 5m, 15m, 30m, 1h, 6h, 12h, daily.

### skill promote

Promotes a Hermes skill to dotagents shared skills. Copies the skill, creates a branch, commits, pushes, and opens a PR.

```bash
dotagents skill promote SKILL_NAME          # by name (searches ~/.hermes/skills/)
dotagents skill promote <category>/<name>   # with category prefix
dotagents skill promote <name> --dry-run    # copy only, skip git/PR
```

The skill is searched in `~/.hermes/skills/` by name (including inside category subdirectories). The promote command:
1. Copies the skill to `skills/<name>/`
2. Creates branch `promote/<name>`
3. Commits as "add <name> skill"
4. Pushes and creates a PR via `gh`

Requires `gh` CLI authenticated with push access to the repo.

For the full promotion workflow (evaluation, pr-triage, merge), use the `skill-promote` Hermes skill.

### doctor --e2e

Runs sync, status, and doctor end to end. It fails on any drift, conflict, or doctor warning and is the canonical full-pipeline verification mode.

```bash
dotagents doctor --e2e
```

## What it manages

- Fixes `~/.agents` if it should point at the repo root and is missing or drifted.
- Configures Hermes to load skills from `~/.agents/skills` through `~/.hermes/config.yaml` when Hermes is a configured target.
- Links Droid global instructions: `~/.factory/AGENTS.md -> ~/.agents/AGENTS.md` (real files conflict).
- Detects agents by checking if their binary is on PATH (`detect` field in config).
- Treats repo skills under `skills/` as the managed set for each detected agent.
- Treats repo hooks under `hooks/`, memory hooks under `memory/hooks/`, and skill hook entrypoints as managed only when declared in `dotagents.yaml`.
- Renders canonical repo roles from `agents/subagents.yaml` to each detected agent's native `agent_root` format where supported:
  - Claude Code: `~/.claude/agents/<name>.md`
  - Codex: `~/.codex/agents/<name>.toml`
  - Factory Droid: `~/.factory/droids/<name>.md`
  - OMP: `~/.omp/agent/agents/<name>.md`
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

- `linkedin` -> `uvx linkedin-scraper-mcp==4.13.2`
- `tavily` -> `npx -y mcp-remote@0.1.38 https://mcp.tavily.com/mcp`

Status and sync rules:

- `dotagents status` reports `mcp managed`, `mcp missing`, and `mcp drifted` alongside skills.
- `dotagents sync` patches only the managed MCP entries for each supported agent.
- Unrelated MCP servers remain untouched.

Per-agent targets:

- Claude Code: `~/.claude.json` -> `mcpServers.<name>`
- Codex: `~/.codex/config.toml` -> `[mcp_servers.<name>]`
- Hermes: `~/.hermes/config.yaml` -> `mcp_servers.<name>`
- Factory Droid: `~/.factory/mcp.json` -> `mcpServers.<name>`
- OMP: `~/.omp/agent/mcp.json` -> `mcpServers.<name>`

Do not symlink whole agent config files. Use targeted patches only. Droid MCP config is patched in-place at `~/.factory/mcp.json`; it is not symlinked.

## Config

Default targets live in:

```bash
dotagents.yaml
```

Use `--agents` only as a one-run override.
