# dotagents

Cross-agent sync CLI for managing shared skills, agent roles, and MCP servers across the primary coding-agent stack: Claude Code, Codex, Factory Droid, Hermes, and Pi/OMP. Amp, OpenClaw, OpenCode, and similar non-primary harnesses are compatibility-only unless explicitly configured.

This repo is the canonical `~/.agents` layer. It detects installed agent platforms, syncs shared skills and MCP entries to each platform's native format, validates drift, and self-tests.

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

- `bittorrent` - manage legal BitTorrent downloads, magnet links, metadata inspection, and client diagnostics.
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
- `tech-search` - gather high-signal opinions from tech communities and blogs on a topic.
- `x-cli` - unofficial CLI for `x` tooling.

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

Dotagents treats plugins as first-party catalog entries in `dotagents.yaml`, not as committed `.codex-plugin`, `.claude-plugin`, `.amp/`, or `.hermes/` runtime directories. A plugin entry records its source format, runtime surfaces, target agents, and review notes:

```yaml
plugins:
  - name: feature-dev
    enabled: false
    source: claude:claude-plugins-official/feature-dev
    format: claude-plugin
    surfaces: [skills, agents, commands, native-plugin]
    agents: [claude-code, codex, hermes, droid, pi]
```

Enabled plugin `skills/` surfaces are discovered from portable plugin source IDs. `codex:<source>/<plugin>` resolves under `DOTAGENTS_CODEX_PLUGIN_ROOT`; `claude:<marketplace>/<plugin>` resolves under `DOTAGENTS_CLAUDE_PLUGIN_ROOT`. For Claude Code, Codex, Factory Droid, and Pi/OMP, `dotagents sync` manages those plugin skills as symlinks in the native skill roots. For Hermes, `dotagents setup` adds the plugin `skills/` directories to `skills.external_dirs`. Amp remains compatibility-only and must be targeted explicitly in a local config if needed.

`dotagents status` prints each plugin's compatibility across known harness descriptors; non-primary harnesses show as `not targeted` unless explicitly configured. `dotagents doctor` validates the catalog and warns when an enabled plugin targets an agent that has no supported surface for it.

Compatibility model:

- `skills` work through managed symlinks for Claude Code/Codex/Factory Droid/Pi and `skills.external_dirs` for Hermes.
- `mcp` works through managed MCP entries.
- `agents` currently renders to Claude Code, Codex, and Droid.
- `hooks` are supported only where dotagents has verified hook config support.
- `native-plugin` is host-specific: `.codex-plugin` stays Codex-native and `.claude-plugin` stays Claude-native.
- `commands` are currently Claude-native unless re-modeled as skills, hooks, MCP, or a repo-owned CLI.

## Agent Integration Status

Dotagents keeps `~/.agents` as the source of truth and adapts each agent through symlinks, targeted config patches, or generated native files. Do not commit agent-specific project runtime directories such as `.amp/` or `.hermes/` to this repo.

| Agent | Shared skills | Native subagents | MCP sync | Hook sync | Root instructions | Integration notes |
|---|---|---|---|---|---|---|
| Claude Code | Symlink mirror to `~/.claude/skills` | Generated to `~/.claude/agents` | `~/.claude/settings.json` | `~/.claude/settings.json` | `CLAUDE.md` shim points to `AGENTS.md` | Full managed mirror for skills, roles, MCP, and supported hooks. |
| Codex | Symlink mirror to `~/.codex/skills` | Generated to `~/.codex/agents` | `~/.codex/config.toml` | `~/.codex/hooks.json` plus `[features].hooks = true` | Reads `AGENTS.md` | Full managed mirror for skills, roles, MCP, and supported hooks. |
| Hermes | Config path to `~/.agents/skills` | Not managed | `~/.hermes/config.yaml` | `~/.hermes/config.yaml` for known lifecycle hooks | Reads configured Hermes context | Uses `skills.external_dirs`; do not mirror into `~/.hermes/skills` because bundled categories can collide. |
| Factory Droid | Symlink mirror to `~/.factory/skills` | Generated to `~/.factory/droids` | `~/.factory/mcp.json` | `~/.factory/settings.json` | `~/.factory/AGENTS.md` symlink | Full managed mirror for skills, roles, MCP, and supported hooks. |
| Pi/OMP | Symlink mirror to `~/.omp/agent/skills` | Not managed | `~/.omp/agent/mcp.json` | Not managed | Reads configured OMP context | Primary OMP target for shared skills, MCP entries, and portable plugin skill surfaces. |

Compatibility-only harness support may remain in the CLI for migration, hook cleanup, trailer stripping, or one-off local configs. Those harnesses are intentionally absent from the canonical `dotagents.yaml` managed target list.


Managed hook declarations live in `dotagents.yaml`. `dotagents sync` may patch supported hook config, but it never approves hook execution. Host-specific hook approval remains manual and lifecycle-sensitive.

## Experimental

`experimental/` holds evaluation notes, landscape comparisons, and alternatives explored. Not necessarily implemented or supported - more about what was tried, what the options are, and why certain choices were made. Reference material for future decisions.
