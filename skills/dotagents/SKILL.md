---
name: dotagents
description: Set up, inspect, and sync a private user-owned agent configuration across Claude Code, Codex, Hermes, Droid, OpenCode, Qwen Code, Pi, and OMP. Use for dotagents setup, status, sync, doctor, skill, MCP, hook, role, memory-tier, or config-root workflows.
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
dotagents status [--verbose] [--agents ...]
dotagents sync [--pull] [--agents ...]
dotagents doctor [--e2e] [--agents ...]
dotagents config
dotagents config serve [--no-open] [--addr 127.0.0.1:8765] [--secure-cookie]
dotagents config validate
dotagents config print
dotagents view [--no-open] [--ssh-host user@host] [--port N] [--host ADDR]
dotagents skill new <name> [--description ...]
dotagents skill list [--agents ...]
dotagents skill info <name>
dotagents skill update [name ...]
dotagents skill promote <name-or-path> [--dry-run]
dotagents publish [--target NAME] [--skills a,b] [--dry-run] [--json] [--yes]
dotagents mcp <list|add|import|remove> [options]
```

`config` is the canonical authoring surface. It edits shared YAML or the
machine-local overlay; effective configuration is read-only. Saves validate
and show a YAML diff, but never run `sync` implicitly. `config serve` binds
only to loopback and uses a session cookie plus CSRF protection. `view` remains
the HarnessKit inspection boundary.

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

The public starter contains `dotagents`, the pinned `grilling` example, six generic roles (`architect` `builder` `general` `researcher` `reviewer` `tester`), and reusable memory scripts. Personal skills, hooks, MCP servers, secrets, and memory data belong only in the private config repository.

Memory tiers:

- `off`: no managed memory hooks.
- `basic` (default): Python-3-only bounded Markdown digests under `$KNOWLEDGE_DIR/sessions/` plus recent context at session start.
- `memsearch`: the indexed SessionStart, Stop, and SessionEnd pipeline; requires `memsearch` on `PATH`.

For Hermes, the `memsearch` tier also keeps the built-in memory files and the
knowledge vault in sync: `sync-vault-to-memory.sh` runs at session start,
`session-end.sh` captures the session digest at finalize, and
`sync-memory-to-vault.sh` exports durable memory at finalize. The generic
`session-start.sh` is not registered for Hermes because its Claude-style
context response is not consumed by Hermes; Hermes injects the synchronized
built-in memory natively instead.

Verify the complete setup with `dotagents doctor --e2e`, then run
`hermes hooks doctor`. Hermes hook approval is host-local and may require one
interactive approval after setup.

## status

Reports each configured harness as detected or not detected and compares the four managed surfaces with native state:

- skills
- MCP servers
- hooks
- agent roles

Missing, drifted, conflicting, stale managed, and unrelated external entries are reported separately.

## sync

Reconciles only configured managed entries. Unrelated native content remains untouched.

For symlink-based harnesses, skills point to canonical directories under `~/.agents/skills`. Hermes uses `skills.external_dirs: ["~/.agents/skills"]`; Qwen Code uses `skills.directories: ["~/.agents/skills"]`. Both consume the canonical tree without creating a duplicate mirror.

Agent roles are canonical Markdown files under `~/.agents/agents/` and render to:

- Claude Code: `~/.claude/agents/<name>.md`
- Codex: `~/.codex/agents/<name>.toml`
- Factory Droid: `~/.factory/droids/<name>.md`
- OpenCode: `~/.config/opencode/agents/<name>.md`
- OMP: `~/.omp/agent/agents/<name>.md`
- Qwen Code: `~/.qwen/agents/<name>.md`

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

## publish

`publish` is the outward analogue of external skills: it pushes canonical skills to a remote skill registry (OpenAI `/v1/skills`) and pins the returned id/version in `dotagents.lock` under `published_skills`. Declare opt-in targets with an explicit skill allowlist in `dotagents.yaml`:

```yaml
publish_targets:
  - name: openai
    kind: openai-skills
    enabled: true
    skills: [jobs, tech-search]
    api_key_env: OPENAI_API_KEY   # key is read from this env var, never inlined
```

`dotagents publish` uploads only allowlisted skills whose bundle content changed since the last run (content-hash idempotency: create, skip if unchanged, new version if changed). It is inert until a target sets `enabled: true`. Use `--dry-run` to preview, `--target`/`--skills` to narrow, `--json` for machine output, `--yes`/`-y` to skip the confirmation prompt. A real upload always prints a US-only / no-Zero-Data-Retention warning first; do not publish skills carrying secrets or private vault content.

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

## view

Launches [HarnessKit](https://github.com/RealZST/HarnessKit) (`hk serve`) as an inspection web UI over every detected harness — skills, MCP, hooks, and configs in one place, with a security audit. The `view` command writes nothing, but HarnessKit's own enable/disable/deploy actions bypass dotagents; treat `view` as inspect/audit and reconcile any HarnessKit changes with `dotagents sync`. Requires `hk` on `PATH` (install HarnessKit separately).

It prints the tokenized URL on its own line and, when running locally, opens it in your default browser. `--no-open` suppresses the browser launch. On a remote host, pass `--ssh-host user@host` (or run inside an SSH session, where it derives the host from `SSH_CONNECTION`) to print a ready `ssh -L` tunnel command instead of auto-opening. Any other flags (`--port`, `--host`, `--no-token`, `--name`) are forwarded to `hk serve`.

```bash
dotagents view                                 # open the inspector locally
dotagents view --no-open --port 7070           # print the URL, do not open a browser
dotagents view --ssh-host me@box --host 0.0.0.0 # remote: print an ssh -L tunnel command
```

## Capability matrix

| Harness | Skills | Roles | MCP | Hooks |
|---|---|---|---|---|
| Claude Code | yes | yes | yes | yes |
| Codex | yes | yes | yes | yes |
| Factory Droid | yes | yes | yes | yes |
| Hermes | yes, config-driven | no | yes | yes |
| OpenCode | yes | yes | yes | no |
| Pi | yes | no | no | no |
| OMP | yes | yes | yes | no |
| Qwen Code | yes, config-driven | yes | yes | yes |

Do not add a surface without a verified native adapter and focused tests.
