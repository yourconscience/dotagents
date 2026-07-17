# Spec: OpenCode harness support

Implementer: builder agent (claude-opus-4-6). Worktree: `.worktrees/opencode-support`, branch `feature/opencode-support` from latest `origin/main` (after PR #118 merges). Open a PR when done; the session lead reviews and merges.

Motivation: Kimi K3 (released 2026-07-16) made OpenCode the go-to open harness alongside Pi and Claude Code; Moonshot ships an official OpenCode guide. Users arriving from that wave should get first-class dotagents support.

## Verified facts (source: docs/plans/launch-2026-07-14/08-opencode-research.md)

All verified against opencode.ai/docs and repo source on 2026-07-17. OpenCode v1.18.3 (2026-07-16), very active.

- Binary on PATH: `opencode`. Config root: `~/.config/opencode/` (honors `$XDG_CONFIG_HOME`). Main config: `opencode.json` (JSONC variant possible; schema `https://opencode.ai/config.json`).
- **Skills**: native `SKILL.md` support, agentskills.io-style frontmatter (`name` required, must match dir name, pattern `^[a-z0-9]+(-[a-z0-9]+)*$`; `description` required). Reads global skills from THREE roots: `~/.config/opencode/skills/`, `~/.claude/skills/`, and `~/.agents/skills/`.
- **Agents (subagent roles)**: Markdown files at `~/.config/opencode/agents/<name>.md`. Filename = agent name; body = system prompt. Frontmatter fields: `description` (required), `mode` (primary|subagent|all), `model` (`provider/model-id` form), `temperature`, `tools`, `color`.
- **MCP**: `mcp` key inside `opencode.json`. Local server shape: `{"type":"local","command":["cmd","arg1",...],"environment":{...},"enabled":true}`. Remote: `{"type":"remote","url":...}`. NOTE: `command` is one array including the binary; env key is `environment`, NOT `env`.
- **Hooks**: no declarative lifecycle hooks. Plugin system = JS/TS modules. Out of scope (see below).
- Instructions: reads project `AGENTS.md` and `~/.config/opencode/AGENTS.md`.

## Design decisions (already made — do not relitigate)

1. **Skills = native read, not mirror.** OpenCode already reads `~/.agents/skills/` directly. When the dotagents config root IS `~/.agents` (the default), sync must NOT copy skills into `~/.config/opencode/skills/` — that would double-list every skill (dual-delivery bug class). The adapter reports skills as natively consumed. When the config root is elsewhere (`DOTAGENTS_HOME`/`--config`), mirror skills into `~/.config/opencode/skills/` like other harnesses.
2. **Doctor duplicate check.** `doctor` warns when a skill exists both in `~/.agents/skills/` and `~/.config/opencode/skills/` under the same name (double listing in OpenCode).
3. **Roles**: render canonical `agents/*.md` into `~/.config/opencode/agents/<name>.md`. Map frontmatter: `description` → `description`; `model`/`effort` have no direct equivalent — omit unless a per-harness override block `opencode:` is present in the canonical role (mirror of the existing `codex:` override pattern; support `model`, `temperature`, `mode`; default `mode: subagent`). Instructions body passes through. Managed-marker convention same as other harnesses so sync can detect drift and setup can skip unmanaged files.
4. **MCP**: merge managed entries into `~/.config/opencode/opencode.json` under `mcp`, translating `command` + `args` → single `command` array and `env` → `environment`. Surgical JSON merge — never touch unmanaged keys or servers. Respect `agents:` targeting (server applies to opencode only when listed).
5. **Hooks: unsupported.** OpenCode's hook surface is a JS plugin API, not declarative config. Per repo invariant ("do not invent unsupported surfaces"), hooks stay `--` for OpenCode in v1. Do not generate plugin files.
6. **Detection**: `detect: opencode`. Default agent entry added to `defaultAgentConfigs()` and `setupSelectableAgentConfigs()`: name `opencode`, skill_root `~/.config/opencode/skills`, agent_root `~/.config/opencode/agents`.
7. **Setup import**: scan existing `~/.config/opencode/agents/*.md` (convert = mostly pass-through with frontmatter normalization) and `opencode.json` MCP servers (reverse-translate `command` array → command+args, `environment` → env) as import candidates, same prompts as other harnesses. Copy-only.
8. **Respect XDG**: resolve config root as `$XDG_CONFIG_HOME/opencode` when set, else `~/.config/opencode`.

## Deliverables

1. Harness adapter in `cmd/dotagents/harness.go` (+ any new opencode-specific file) implementing the above.
2. Setup/import/scan support; prune-guard integration comes free via existing report flow — verify it triggers for opencode.
3. Tests, following existing per-harness test patterns:
   - default config entry + detection
   - role render (frontmatter mapping incl. `opencode:` override + default mode)
   - MCP merge: fresh file, existing unmanaged servers preserved, command/env translation both directions (import)
   - skills native-read path: no mirror when root == ~/.agents; mirror when custom root
   - doctor duplicate-skill warning
   - setup import scan for agents + MCP
4. Docs: README harness table row `OpenCode | yes | yes | yes | -- `(with a note that skills are read natively from `~/.agents`); `docs/harness-map.html` — move OpenCode from compatibility queue to a primary card + matrix row, regenerate `docs/harness-map.png` if a headless browser is available (Brave at `/Applications/Brave Browser.app/Contents/MacOS/Brave Browser` on this machine; window 1400x1560), otherwise note it in the PR; landing `docs/site/index.html` harness grid + table row.
5. `dotagents.yaml` template: nothing to add (agents are detected, not templated).

## Acceptance

- `go build ./cmd/dotagents` and full `go test ./...` pass.
- `dotagents status`/`sync`/`doctor` behave correctly on a machine with opencode binary faked on PATH (tests cover this; no live install required).
- No changes to unrelated harness behavior (existing tests untouched except where a shared fixture legitimately grows).
- Single PR from `feature/opencode-support`, short single-line commits, no bot trailers. Do NOT merge — open the PR and stop.

## Out of scope

- OpenCode plugins/hooks, project-level `.opencode/`, remote MCP servers (only local stdio in v1; remote entries in opencode.json must be preserved untouched), model/provider config (Kimi K3 setup is the user's business), Amp/OpenClaw.
