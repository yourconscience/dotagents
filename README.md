# dotagents

Cross-agent sync CLI for managing shared skills, agent roles, and MCP servers across the primary coding-agent stack: Claude Code, Codex, Factory Droid, Hermes, and Pi/OMP. Amp, OpenClaw, OpenCode, and similar non-primary harnesses are compatibility-only unless explicitly configured.

This repo is the canonical `~/.agents` layer. It detects installed agent platforms, syncs shared skills and MCP entries to each platform's native format, validates drift, and self-tests.

![dotagents harness map](./docs/harness-map.png)

[Open the full harness map.](./docs/harness-map.html)

## Agent instructions

[`AGENTS.md`](./AGENTS.md) is the canonical instruction file. [`CLAUDE.md`](./CLAUDE.md) is only a compatibility shim for agents that look for Claude-style project memory.

## CLI

Install the repo-owned CLI:

```bash
go install ./cmd/dotagents
```

Ensure the Go install directory is on `PATH`. If `go env GOBIN` is non-empty, add that directory; otherwise add `$(go env GOPATH)/bin`.

After that, use `dotagents` directly:

```bash
dotagents status
dotagents deps check
```

## Agents

Reusable agent role definitions for agent-native subagents. Canonical roles live in `agents/*.yaml`; `dotagents sync` renders them to each configured native format:

- Claude Code: `~/.claude/agents/<name>.md`
- Codex: `~/.codex/agents/<name>.toml`
- Factory Droid: `~/.factory/droids/<name>.md`

- `architect` - designs system architecture, telemetry schemas, and technical plans. Sonnet, read + write.
- `builder` - implements code changes following specs or architect designs. Sonnet, read + write.
- `researcher` - investigates codebases, APIs, repos, and web sources. Sonnet, read + write + web.
- `reviewer` - reviews code against specs, finds bugs and security issues. Sonnet, read-only.

Reference these from TeamCreate teammates, Claude Code subagent types, or Codex native subagent roles. See `skills/spawn/SKILL.md` for usage patterns.

## Skills

- `cmux` - control cmux workspaces, panes, terminal/browser surfaces, markdown viewers, and visible agent workspaces.
- `tmux` - generic tmux reference for sessions, windows, panes, screen capture, and input.
- `dotagents` - inspect and sync the repo-owned skill links across supported coding agents.
- `grill-me` - pressure-test a plan one question at a time until scope and decisions are concrete.
- `gws` - Google Workspace workflows. On Hermes, prefer the bundled native `google-workspace` skill; this repo's `skills/gws` remains the shared source for Claude Code/Codex and CLI helpers.
- `humanizer` - final-pass rewriting for concise writing that keeps the user's voice.
- `jobs` - track job search pipeline, analyze fit for postings, generate interview quizzes, grade answers.
- `remote-access` - search local Droid/Codex sessions and send scoped continuation instructions through the Mac bridge from mobile.
- `pr-triage` - inspect PR failures and unresolved review threads, then drive a single fix-commit-push loop.
- `repo-eval` - find, triage, and deep-evaluate GitHub repos for a given need.
- `spec` - produce a small `SPEC.md` for complex or ambiguous work before implementation.
- `spawn` - spawn and manage Claude Code agent teams with model routing and cmux integration.
- `tg` - read Telegram chats, search messages, and list dialogs through the read-only `tg` CLI.
- `tech-search` - gather high-signal opinions from tech communities and blogs on a topic.
- `tg` - read Telegram chats, search messages, and list dialogs via the read-only `tg` CLI.
- `x-cli` - unofficial CLI for `x` tooling.
- `x-sim` - offline X audience simulation for draft tweets and handle positioning.

## External Skills

Skills from external git repos can be synced alongside local skills. Declare sources in `dotagents.yaml` when needed:

```yaml
external_skills:
  - url: https://github.com/example/shared-skills
    skill_dir: skill
    branch: main
```

`dotagents sync` clones or updates each repo into `~/.agents/external/<repo-name>/` and symlinks discovered skills into agent skill roots. `dotagents status` shows external sources with their commit hash. `dotagents doctor` validates that clones exist and contain valid skills.

## Plugins

Dotagents treats third-party plugins as first-party catalog entries in `dotagents.yaml`, not as committed `.codex-plugin`, `.amp/`, or `.hermes/` runtime directories. (The repo's own `.claude-plugin/` manifests are the one exception; see "Installing this repo as a Claude Code plugin" below.) A plugin entry records its source format, runtime surfaces, target agents, and review notes:

```yaml
plugins:
  - name: feature-dev
    enabled: false
    source: claude:claude-plugins-official/feature-dev
    format: claude-plugin
    surfaces: [skills, agents, commands, native-plugin]
    agents: [claude-code, codex, hermes, droid, pi]
```

Enabled plugin `skills/` surfaces are discovered from portable plugin source IDs. `codex:<source>/<plugin>` resolves under `DOTAGENTS_CODEX_PLUGIN_ROOT`; `claude:<marketplace>/<plugin>` resolves under `DOTAGENTS_CLAUDE_PLUGIN_ROOT`. For Codex, Factory Droid, and Pi/OMP, `dotagents sync` manages those plugin skills as symlinks in the native skill roots. Claude Code uses either symlink sync or this repo's native Claude plugin based on `agents[].delivery`. For Hermes, `dotagents setup` adds the plugin `skills/` directories to `skills.external_dirs`. Amp remains compatibility-only and must be targeted explicitly in a local config if needed.

`dotagents status` prints each plugin's compatibility across known harness descriptors; non-primary harnesses show as `not targeted` unless explicitly configured. `dotagents doctor` validates the catalog and warns when an enabled plugin targets an agent that has no supported surface for it.

Compatibility model:

- `skills` work through managed symlinks for Claude Code/Codex/Factory Droid/Pi and `skills.external_dirs` for Hermes.
- `mcp` works through managed MCP entries.
- `agents` currently renders to Claude Code, Codex, and Droid.
- `hooks` are supported only where dotagents has verified hook config support.
- `native-plugin` is host-specific: `.codex-plugin` stays Codex-native and `.claude-plugin` stays Claude-native.
- `commands` are currently Claude-native unless re-modeled as skills, hooks, MCP, or a repo-owned CLI.

### Installing this repo as a Claude Code plugin

The repo doubles as a self-hosted Claude Code plugin and single-plugin marketplace via `.claude-plugin/plugin.json` and `.claude-plugin/marketplace.json`:

```
/plugin marketplace add yourconscience/dotagents
/plugin install dotagents@yourconscience
```

The repo is private, so `marketplace add` requires working GitHub git auth (ssh or `gh auth`) on the machine.

The plugin ships every skill under `skills/` (namespaced as `/dotagents:<skill>`) plus the four subagent roles. The roles are rendered from `agents/*.yaml` into `agents/*.md` (beside the sources) by `dotagents render`; Claude Code auto-discovers them from the top-level `agents/` directory. `dotagents doctor` and CI tests fail (`plugin agents` check) when the rendered copies drift from the YAML.

Plugin skills are byte-identical to the symlink-synced ones - same directories, same repo. A machine should use exactly one Claude Code delivery channel:

```yaml
agents:
  - name: claude-code
    delivery: sync   # default: dotagents sync manages ~/.claude/skills and ~/.claude/agents
```

Use the CLI wrapper to switch Claude Code to plugin delivery:

```bash
dotagents plugin add
```

That command runs the Claude plugin install flow, sets `delivery: plugin` for `claude-code`, and prunes dotagents-managed symlinks and generated Claude agent files so skills do not appear twice (`/tg` and `/dotagents:tg`). `dotagents doctor` includes a `claude delivery` check that fails when `delivery: plugin` is set but `dotagents@yourconscience` is not installed, or when plugin delivery still has managed sync artifacts.

Use this to return Claude Code to symlink sync:

```bash
dotagents plugin remove
```

That uninstalls the Claude plugin, removes the marketplace entry, sets `delivery: sync`, and runs `dotagents sync --agents=claude-code`. Plugin installs snapshot the repo at install time; consumers pick up new skills with `/plugin update`, unlike the always-live symlinks. The plugin manifest intentionally omits a fixed `version` so Claude Code uses the git commit SHA and every new commit can be updated.

**Why Claude Code only.** Claude Code is the one supported agent whose native plugin format fits a shared-root skill library: it auto-discovers `skills/` and `agents/` from the plugin (repo) root. Codex has a plugin system too, but its plugins must live in a subdirectory with a *real, copied* `skills/` inside the plugin directory - it ignores symlinks and rejects a plugin at the marketplace root (verified against `codex 0.136.0`). Bundling our 31MB shared `skills/` into a committed subdir would mean a second source of truth, so Codex - like Droid, Amp, Hermes, and Pi - consumes dotagents skills through `dotagents sync`, not a plugin. `SKILL.md` directories remain the genuinely portable cross-tool convention.

## Agent Integration Status

Dotagents keeps `~/.agents` as the source of truth and adapts each agent through symlinks, targeted config patches, or generated native files. Do not commit agent-specific project runtime directories such as `.amp/` or `.hermes/` to this repo.

| Agent | Shared skills | Native subagents | MCP sync | Hook sync | Root instructions | Integration notes |
|---|---|---|---|---|---|---|
| Claude Code | `delivery: sync` symlink mirror to `~/.claude/skills`; `delivery: plugin` via `dotagents@yourconscience` | `delivery: sync` generated to `~/.claude/agents`; `delivery: plugin` from plugin `agents/*.md` | `~/.claude/settings.json` | `~/.claude/settings.json` | `CLAUDE.md` shim points to `AGENTS.md` | Use exactly one skill/role delivery channel; MCP and supported hooks remain dotagents-managed. |
| Codex | Symlink mirror to `~/.codex/skills` | Generated to `~/.codex/agents` | `~/.codex/config.toml` | `~/.codex/hooks.json` plus `[features].hooks = true` | Reads `AGENTS.md` | Full managed mirror for skills, roles, MCP, and supported hooks. |
| Hermes | Config path to `~/.agents/skills` | Not managed | `~/.hermes/config.yaml` | `~/.hermes/config.yaml` for known lifecycle hooks | Reads configured Hermes context | Uses `skills.external_dirs`; do not mirror into `~/.hermes/skills` because bundled categories can collide. |
| Factory Droid | Symlink mirror to `~/.factory/skills` | Generated to `~/.factory/droids` | `~/.factory/mcp.json` | `~/.factory/settings.json` | `~/.factory/AGENTS.md` symlink | Full managed mirror for skills, roles, MCP, and supported hooks. |
| Pi/OMP | Symlink mirror to `~/.omp/agent/skills` | Not managed | `~/.omp/agent/mcp.json` | Not managed | Reads configured OMP context | Primary OMP target for shared skills, MCP entries, and portable plugin skill surfaces. |

Compatibility-only harness support may remain in the CLI for migration, hook cleanup, trailer stripping, or one-off local configs. Those harnesses are intentionally absent from the canonical `dotagents.yaml` managed target list.


Managed hook declarations live in `dotagents.yaml`. `dotagents sync` may patch supported hook config, but it never approves hook execution. Host-specific hook approval remains manual and lifecycle-sensitive.

## Experimental

`experimental/` holds evaluation notes, landscape comparisons, and alternatives explored. Not necessarily implemented or supported - more about what was tried, what the options are, and why certain choices were made. Reference material for future decisions.
