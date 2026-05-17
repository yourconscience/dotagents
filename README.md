# dotagents

Private repo for my shared authored `~/.agents` layer.

This repo is under active development and currently holds shared rules, cross-tool skills, and small agent-native config parity such as managed MCP entries for Amp, Claude Code, Codex, Hermes Agent, Factory Droid, and OpenClaw.

## Agent instructions

[`AGENTS.md`](./AGENTS.md) is the canonical instruction file. [`CLAUDE.md`](./CLAUDE.md) is only a compatibility shim for agents that look for Claude-style project memory.

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
- `gws` - Google Workspace workflows. On Hermes, prefer the bundled native `google-workspace` skill; this repo's `skills/gws` remains the shared source for Claude Code/Codex/OpenClaw and CLI helpers.
- `humanizer` - final-pass rewriting for concise writing that keeps the user's voice.
- `jobs` - track job search pipeline, analyze fit for postings, generate interview quizzes, grade answers.
- `remote-access` - search local Droid/Codex sessions and send scoped continuation instructions through the Mac bridge from mobile.
- `pr-triage` - inspect PR failures and unresolved review threads, then drive a single fix-commit-push loop.
- `repo-eval` - find, triage, and deep-evaluate GitHub repos for a given need.
- `spec` - produce a small `SPEC.md` for complex or ambiguous work before implementation.
- `spawn` - spawn and manage Claude Code agent teams with model routing and cmux integration.
- `tech-search` - gather high-signal opinions from tech communities and blogs on a topic.
- `x-cli` - unofficial CLI for `x` tooling.

Amp uses `~/.agents/skills` through `amp.skills.path` in `~/.config/amp/settings.json`. `dotagents setup` patches that setting and `dotagents sync` keeps managed MCP entries under `amp.mcpServers`.

Dotagents integrations should stay repo-owned and agent-agnostic: shared skills under `skills/`, hook scripts under `memory/hooks/`, knowledge sync under `memory/`, and subagent roles under `agents/`. Do not commit agent-specific project directories such as `.amp/` or `.hermes/` to this repo; wire each agent through targeted config patches or generated native files instead.

Hermes uses `~/.agents/skills` through `skills.external_dirs` in `~/.hermes/config.yaml`. Do not mirror repo skills into `~/.hermes/skills`, because Hermes already ships bundled categorized skills there and name collisions are easy.

## Experimental

`experimental/` holds evaluation notes, landscape comparisons, and alternatives explored. Not necessarily implemented or supported - more about what was tried, what the options are, and why certain choices were made. Reference material for future decisions.
