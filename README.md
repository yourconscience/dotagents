# dotagents

Private repo for my shared authored `~/.agents` layer.

This repo is under active development and currently holds shared rules, cross-tool skills, and small agent-native config parity such as managed MCP entries for Claude Code, Codex, Hermes Agent, and OpenClaw.

## Agent instructions

[`AGENTS.md`](./AGENTS.md) is the canonical instruction file. [`CLAUDE.md`](./CLAUDE.md) is only a compatibility shim for agents that look for Claude-style project memory.

## Agents

Reusable agent role definitions for agent-native subagents. Canonical roles live in `agents/*.yaml`; `dotagents sync` renders them to each configured native format:

- Claude Code: `~/.claude/agents/<name>.md`
- Codex: `~/.codex/agents/<name>.toml`

Default model is `gpt-5.5` via Codex (personal-research uses `gpt-5.5-mini`). Sonnet is the Claude Code fallback only - rendered into the Claude Code subagent file so the role still works there, but Codex/GPT-5.5 is the primary execution path.

- `architect` - designs system architecture, telemetry schemas, and technical plans. Read + write.
- `builder` - implements code changes following specs or architect designs. Read + write.
- `researcher` - investigates codebases, APIs, repos, and web sources. Read + write + web.
- `reviewer` - reviews code against specs, finds bugs and security issues. Read-only.
- `personal-research` - non-coding research with citations to primary sources. Web + write. (`gpt-5.5-mini`)

Reference these from TeamCreate teammates, Claude Code subagent types, or Codex native subagent roles. See `skills/spawn/SKILL.md` for usage patterns.

## Skills

- `cmux` - reference for cmux CLI subcommands: workspace management, screen reading, browser automation, agent teams.
- `dotagents` - inspect and sync the repo-owned skill links across supported coding agents.
- `grill-me` - pressure-test a plan one question at a time until scope and decisions are concrete.
- `google-workspace` (`gws`) - Google Workspace workflows. On Hermes, prefer the bundled native `google-workspace` skill; this repo's `skills/gws` remains the shared source for Claude Code/Codex/OpenClaw and CLI helpers.
- `jobs` - track job search pipeline, analyze fit for postings, generate interview quizzes, grade answers.
- `omx` - spawn oh-my-codex in a detached tmux session and delegate a substantial task to it.
- `pr-triage` - inspect PR failures and unresolved review threads, then drive a single fix-commit-push loop.
- `repo-eval` - find, triage, and deep-evaluate GitHub repos for a given need.
- `spec` - produce a small `SPEC.md` for complex or ambiguous work before implementation.
- `spawn` - spawn and manage Claude Code agent teams with model routing and cmux integration.
- `tech-search` - gather high-signal opinions from tech communities and blogs on a topic.
- `x-cli` - unofficial CLI for `x` tooling.

Hermes uses `~/.agents/skills` through `skills.external_dirs` in `~/.hermes/config.yaml`. Do not mirror repo skills into `~/.hermes/skills`, because Hermes already ships bundled categorized skills there and name collisions are easy.

## Experimental

`experimental/` holds evaluation notes, landscape comparisons, and alternatives explored. Not necessarily implemented or supported - more about what was tried, what the options are, and why certain choices were made. Reference material for future decisions.
