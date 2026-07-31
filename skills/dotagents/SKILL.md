---
name: dotagents
description: Set up, inspect, and sync a private user-owned agent configuration across Claude Code, Codex, Hermes, Droid, Pi, and OMP. Use for dotagents setup, status, sync, doctor, skill, MCP, hook, role, memory-tier, or config-root workflows.
---

# dotagents

`dotagents` is a public Go CLI. The user's canonical configuration is a separate private directory or repository.

Config precedence:

1. `--config /path/to/dotagents.yaml`
2. `$DOTAGENTS_HOME/dotagents.yaml`
3. `~/.agents/dotagents.yaml`

Never infer configuration from the current project. Never replace a user's config root with a public checkout.

## Commands

```bash
dotagents setup [--memory off|basic|memsearch] [--agents ...] [--yes] [--dry-run] [--json]
dotagents status [--agents ...]
dotagents sync [--pull] [--agents ...]
dotagents doctor [--e2e] [--agents ...]
dotagents skill new <name> [--description ...]
dotagents skill update [name ...]
dotagents skill promote <name-or-path> [--dry-run]
dotagents mcp <list|add|import|remove> [options]
```

Run `dotagents help --all` for maintenance commands and compatibility aliases. Do not use hidden aliases in new scripts or documentation.

## setup

First-run setup:

1. Creates the canonical config root when absent.
2. Extracts missing public starter assets without overwriting user files.
3. Detects supported harness binaries.
4. Scans native skill, role, and MCP locations.
5. Shows an interactive review screen: share, keep harness-specific, or skip per item; items identical across harnesses are shared automatically. Imports are copy or conversion only; originals remain untouched.
6. In non-interactive runs it falls back to sequential prompts; `--yes` imports everything without prompting, `--dry-run` prints candidates and exits without changes, `--json` emits the detection result and exits.
7. Registers the chosen memory tier.
8. Patches only the required native harness settings.
9. Runs the first sync.

The public starter contains `dotagents`, the pinned `grilling` example, five generic roles, and reusable memory scripts. Personal skills, hooks, MCP servers, secrets, and memory data belong only in the private config repository.

Memory tiers:

- `off`: no managed memory hooks.
- `basic` (default): Python-3-only bounded Markdown digests under `$KNOWLEDGE_DIR/sessions/` plus recent context at session start.
- `memsearch`: the indexed SessionStart, Stop, and SessionEnd pipeline; requires `memsearch` on `PATH`.

## status

Reports each configured harness as detected or not detected and compares the four managed surfaces with native state:

- skills
- MCP servers
- hooks
- agent roles

Missing, drifted, conflicting, stale managed, and unrelated external entries are reported separately.

## sync

Reconciles only configured managed entries. Unrelated native content remains untouched.

For symlink-based harnesses, skills point to canonical directories under `~/.agents/skills`. Hermes uses `skills.external_dirs: ["~/.agents/skills"]` instead of mirroring into its bundled skill tree.

Agent roles are canonical Markdown files under `~/.agents/agents/` and render to:

- Claude Code: `~/.claude/agents/<name>.md`
- Codex: `~/.codex/agents/<name>.toml`
- Factory Droid: `~/.factory/droids/<name>.md`
- OMP: `~/.omp/agent/agents/<name>.md`

Pi has a managed skill root only. OMP is a separate target with skills, roles, and MCP support.

For MCP servers, sync patches only named canonical entries and preserves unrelated native servers. Import redacts literal environment values to `${KEY}` references; list output never prints values.

For hooks, sync registers only declared entries on harnesses with verified hook support. Host-local review and approval state remains outside dotagents; Hermes keys first-use consent by the exact event and command and reports script mtime drift through `hermes hooks doctor`.

`sync --pull` runs `git pull --ff-only` in the private canonical repository before reconciliation.

## External skills

Declare external Git sources in the private `dotagents.yaml`:

```yaml
external_skills:
  - url: https://github.com/example/shared-skills
    branch: main
    skill_dirs: [engineering/alpha, productivity/beta]
    materialize: true
```

`dotagents.lock` records exact commits and materialized ownership. `sync` repairs drift to the pin; `skill update` explicitly advances it. `doctor` audits external source content and fails on materialization drift.

## MCP management

```bash
dotagents mcp list
dotagents mcp add local --command uvx --arg pkg@1.2.3 --env KEY=value
dotagents mcp import claude-code local --agents=codex,hermes,droid,omp
dotagents sync
dotagents mcp remove local
```

Use versions verified against the package registry. Keep secrets in environment variables or host-local native config, never canonical YAML.

## doctor --e2e

Runs sync, status, and doctor as one health check. It fails on drift, conflicts, invalid external pins, unsupported managed claims, or other doctor errors.

```bash
dotagents doctor --e2e
```

## Capability matrix

| Harness | Skills | Roles | MCP | Hooks |
|---|---|---|---|---|
| Claude Code | yes | yes | yes | yes |
| Codex | yes | yes | yes | yes |
| Factory Droid | yes | yes | yes | yes |
| Hermes | yes, config-driven | no | yes | yes |
| Pi | yes | no | no | no |
| OMP | yes | yes | yes | no |

Do not add a surface without a verified native adapter and focused tests.
