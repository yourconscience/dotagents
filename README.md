# dotagents

Cross-agent sync CLI for managing shared skills, agent roles, and MCP servers across Claude Code, Codex, Factory Droid, Amp, Hermes, and OpenClaw from one YAML config.

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
- `remote-access` - mobile-friendly agent access: prefer Untether for Telegram control of Claude Code/Codex/OpenCode/Pi/Gemini/Amp, and use the Mac bridge for read-only local Droid/Codex inspection from Hermes.
- `pr-triage` - inspect PR failures and unresolved review threads, then drive a single fix-commit-push loop.
- `repo-eval` - find, triage, and deep-evaluate GitHub repos for a given need.
- `spec` - produce a small `SPEC.md` for complex or ambiguous work before implementation.
- `spawn` - spawn and manage Claude Code agent teams with model routing and cmux integration.
- `tech-search` - gather high-signal opinions from tech communities and blogs on a topic.
- `x-cli` - unofficial CLI for `x` tooling.

## External Skills

Skills from external git repos can be synced alongside local skills. Declare sources in `dotagents.yaml`:

```yaml
external_skills:
  - url: https://github.com/yourconscience/dotknow
    skill_dir: skill
    branch: main
```

`dotagents sync` clones or updates each repo into `~/.agents/external/<repo-name>/` and symlinks discovered skills into agent skill roots. `dotagents status` shows external sources with their commit hash. `dotagents doctor` validates that clones exist and contain valid skills.

## Agent Integration Status

Dotagents keeps `~/.agents` as the source of truth and adapts each agent through symlinks, targeted config patches, or generated native files. Do not commit agent-specific project runtime directories such as `.amp/` or `.hermes/` to this repo.

| Agent | Shared skills | Native subagents | MCP sync | Hook sync | Root instructions | Integration notes |
|---|---|---|---|---|---|---|
| Claude Code | Symlink mirror to `~/.claude/skills` | Generated to `~/.claude/agents` | `~/.claude/settings.json` | `~/.claude/settings.json` | `CLAUDE.md` shim points to `AGENTS.md` | Full managed mirror for skills, roles, MCP, and supported hooks. |
| Codex | Symlink mirror to `~/.codex/skills` | Generated to `~/.codex/agents` | `~/.codex/config.toml` | Not managed | Reads `AGENTS.md` | Full managed mirror for skills and roles; hooks are not managed until Codex exposes a supported hook surface. |
| Amp | Config path to `~/.agents/skills` | Not managed | Amp settings `amp.mcpServers` | Not managed | Reads `AGENTS.md` | Uses `amp.skills.path`; patches an existing ignored workspace `.amp/settings.*` only when Amp would give it precedence. |
| Hermes | Config path to `~/.agents/skills` | Not managed | `~/.hermes/config.yaml` | `~/.hermes/config.yaml` for known lifecycle hooks | Reads configured Hermes context | Uses `skills.external_dirs`; do not mirror into `~/.hermes/skills` because bundled categories can collide. |
| Factory Droid | Symlink mirror to `~/.factory/skills` | Generated to `~/.factory/droids` | `~/.factory/mcp.json` | Not managed | `~/.factory/AGENTS.md` symlink | Full managed mirror for skills and roles. |
Managed hook declarations live in `dotagents.yaml`. `dotagents sync` may patch supported hook config, but it never approves hook execution. Host-specific hook approval remains manual and lifecycle-sensitive.

## Experimental

`experimental/` holds evaluation notes, landscape comparisons, and alternatives explored. Not necessarily implemented or supported - more about what was tried, what the options are, and why certain choices were made. Reference material for future decisions.
