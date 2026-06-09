# dotagents

Cross-agent sync CLI for managing shared skills, agent roles, and MCP servers across Claude Code, Codex, Factory Droid, Amp, Hermes, and OpenClaw from one YAML config.

This repo is the canonical `~/.agents` layer. It detects installed agent platforms, syncs shared skills and MCP entries to each platform's native format, validates drift, and self-tests.

## Agent instructions

[`AGENTS.md`](./AGENTS.md) is the canonical instruction file. [`CLAUDE.md`](./CLAUDE.md) is only a compatibility shim for agents that look for Claude-style project memory.

## Install

Prebuilt binaries for macOS and Linux (amd64/arm64) are attached to [GitHub Releases](https://github.com/yourconscience/dotagents/releases). With Go installed:

```bash
go install github.com/yourconscience/dotagents/cmd/dotagents@latest
```

Or from a clone:

```bash
go install ./cmd/dotagents
```

Ensure the Go install directory is on `PATH`. If `go env GOBIN` is non-empty, add that directory; otherwise add `$(go env GOPATH)/bin`.

After that, use `dotagents` directly:

```bash
dotagents status
dotagents deps check
```

Releases are cut by pushing a `v*` tag; CI runs GoReleaser, which builds the archives and publishes the GitHub Release.

## Alternatives

How dotagents compares to other cross-agent config sync tools:

| | dotagents | [skillshare](https://github.com/runkids/skillshare) | [vsync](https://github.com/nicepkg/vsync) | [agents-cli](https://github.com/amtiYo/agents) |
|---|---|---|---|---|
| Skills sync | yes (symlinks + config-driven dirs) | yes | yes | yes |
| MCP sync | yes | no | yes | yes |
| Hooks sync | yes (Claude Code, Codex, Hermes, Droid) | no | no | no |
| Native subagent roles | yes (Claude Code, Codex, Droid) | agents as files | yes | no |
| Plugin catalog | yes (first-party `dotagents.yaml` entries) | no | no | no |
| External skill pinning | yes (`dotagents.lock`) | version tracking | no | no |
| Skill security audit | yes (`dotagents doctor`) | yes | no | no |
| Local private overlay | yes (`dotagents.local.yaml`) | no | no | no |
| Target agents | Claude Code, Codex, Amp, Hermes, Factory Droid, Pi/OpenClaw | Claude Code, Codex, Cursor, Gemini, 60+ | Claude Code, Cursor, OpenCode, Codex | Codex, Claude Code, Gemini CLI, Cursor, Copilot, others |
| Language | Go | Go | TypeScript | TypeScript |

dotagents focuses on the post-IDE agent stack (Hermes, Amp, Droid, OpenClaw/Pi alongside Claude Code and Codex) and on syncing the full surface - skills, MCP, hooks, roles, plugins, root instructions - from one canonical `~/.agents` layer.

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
- `tg` - read Telegram chats, search messages, and list dialogs via the `tg` CLI.
- `x-cli` - unofficial CLI for `x` tooling.

## Installing these skills without dotagents

The repo doubles as a [Claude Code plugin marketplace](https://code.claude.com/docs/en/discover-plugins): `.claude-plugin/marketplace.json` exposes the portable skills (`tech-search`, `grill-me`, `humanizer`, `repo-eval`, `spec`, `pr-triage`, `tmux`) as single-skill plugins.

```text
/plugin marketplace add yourconscience/dotagents
/plugin install tech-search@dotagents
```

For any agent managed by dotagents, consume the same skills as an external source with a `skills` allowlist:

```yaml
external_skills:
  - url: https://github.com/yourconscience/dotagents
    skill_dir: skills
    branch: main
    skills: [tech-search, grill-me, humanizer, repo-eval, spec, pr-triage, tmux]
```

Other sync tools that install skills from a git repo (e.g. skillshare) can point at the `skills/` directory directly.

## External Skills

Skills from external git repos can be synced alongside local skills. Declare sources in `dotagents.yaml` when needed:

```yaml
external_skills:
  - url: https://github.com/example/shared-skills
    skill_dir: skill
    branch: main
    skills: [alpha, beta] # optional allowlist; omit to take every skill
```

`dotagents sync` clones or updates each repo into `~/.agents/external/<repo-name>/` and symlinks discovered skills into agent skill roots. `dotagents status` shows external sources with their commit hash. `dotagents doctor` validates that clones exist and contain valid skills.

External sources are pinned in `dotagents.lock` (commit this file): the first sync records each source's commit, and later syncs keep the source at the pinned commit instead of silently tracking the branch. `dotagents external list` shows pin state; `dotagents external update [name ...]` moves sources to the latest branch head and rewrites the lock. `dotagents doctor` warns when a source is unpinned or its cache drifts from the lock, and runs a content audit over external skills that flags risky patterns (pipe-to-shell installs, base64-decode-to-shell, prompt-injection phrasing, credential paths) for human review.

## Local overlay

`dotagents.local.yaml` next to `dotagents.yaml` (gitignored) holds personal additions that should stay out of public git: extra agents, external skill sources, MCP servers, hooks, or plugin entries. Entries merge by name (external sources by repo name); a matching name replaces the public entry wholesale, everything else is appended.

## Plugins

Dotagents treats plugins as first-party catalog entries in `dotagents.yaml`, not as committed `.codex-plugin`, `.claude-plugin`, `.amp/`, or `.hermes/` runtime directories. A plugin entry records its source format, runtime surfaces, target agents, and review notes:

```yaml
plugins:
  - name: feature-dev
    enabled: false
    source: claude:claude-plugins-official/feature-dev
    format: claude-plugin
    surfaces: [skills, agents, commands, native-plugin]
    agents: [claude-code, codex, amp, hermes, droid]
```

Enabled plugin `skills/` surfaces are discovered from portable plugin source IDs. `codex:<source>/<plugin>` resolves under `DOTAGENTS_CODEX_PLUGIN_ROOT`; `claude:<marketplace>/<plugin>` resolves under `DOTAGENTS_CLAUDE_PLUGIN_ROOT`. For Claude Code, Codex, and Factory Droid, `dotagents sync` manages those plugin skills as symlinks in the native skill roots. For Hermes, `dotagents setup` adds the plugin `skills/` directories to `skills.external_dirs`. Amp remains compatibility-only until its plugin surfaces are deliberately enabled.

`dotagents status` prints each plugin's compatibility across Claude Code, Codex, Amp, Hermes, and Droid. `dotagents doctor` validates the catalog and warns when an enabled plugin targets an agent that has no supported surface for it.

Compatibility model:

- `skills` work through managed symlinks for Claude Code/Codex/Factory Droid and `skills.external_dirs` for Hermes.
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
| Amp | Config path to `~/.agents/skills` | Not managed | Amp settings `amp.mcpServers` | Not managed | Reads `AGENTS.md` | Uses `amp.skills.path`; patches an existing ignored workspace `.amp/settings.*` only when Amp would give it precedence. |
| Hermes | Config path to `~/.agents/skills` | Not managed | `~/.hermes/config.yaml` | `~/.hermes/config.yaml` for known lifecycle hooks | Reads configured Hermes context | Uses `skills.external_dirs`; do not mirror into `~/.hermes/skills` because bundled categories can collide. |
| Factory Droid | Symlink mirror to `~/.factory/skills` | Generated to `~/.factory/droids` | `~/.factory/mcp.json` | `~/.factory/settings.json` | `~/.factory/AGENTS.md` symlink | Full managed mirror for skills, roles, MCP, and supported hooks. |


Managed hook declarations live in `dotagents.yaml`. `dotagents sync` may patch supported hook config, but it never approves hook execution. Host-specific hook approval remains manual and lifecycle-sensitive.

## Experimental

`experimental/` holds evaluation notes, landscape comparisons, and alternatives explored. Not necessarily implemented or supported - more about what was tried, what the options are, and why certain choices were made. Reference material for future decisions.
