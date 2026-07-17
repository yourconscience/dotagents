# OpenCode Config Surfaces — Fact Sheet (for sync adapter)

Researched: 2026-07-17. Target: `github.com/sst/opencode`, docs `opencode.ai/docs`.
All facts below verified from official docs (`opencode.ai/docs/*`) and the repo source
(`packages/web/src/content/docs/*.mdx`, `packages/plugin/src/index.ts`) on the `dev` branch.

Latest release at research time: **v1.18.3**, published **2026-07-16T15:34:33Z**
(source: `gh api repos/sst/opencode/releases/latest`). Project is highly active
(near-daily releases).

---

## 0. Config root, binary, and precedence

- **Global config dir:** `~/.config/opencode/` (respects `$XDG_CONFIG_HOME`, so
  `$XDG_CONFIG_HOME/opencode/` when set). Custom override via `OPENCODE_CONFIG` env var.
- **Global config file:** `~/.config/opencode/opencode.json` (also `opencode.jsonc` — JSONC
  with comments is supported).
- **Project config file:** `opencode.json` (or `.jsonc`) in the project root.
- **Binary name to detect on PATH:** `opencode`.
- **JSON schema URL:** `https://opencode.ai/config.json` (put in `$schema`). TUI settings use a
  separate `tui.json` file with schema `https://opencode.ai/tui.json`.
- **Merge semantics:** configs are merged (not replaced); later overrides earlier for
  conflicting keys. Load order (later wins): remote config -> global config ->
  `OPENCODE_CONFIG` custom path -> project config -> managed/enterprise settings.
- Top-level config keys include: `model`, `provider`, `agent`, `permission`, `tools`,
  `mcp`, `plugin`, `instructions`, `formatter`, `lsp`, `theme`, `keybinds`, `server`, `shell`.
- Source: <https://opencode.ai/docs/config/>

---

## 1. Skills (SKILL.md)

**Supported — yes.** OpenCode natively loads `SKILL.md`-style skills and explicitly reads the
Claude and `.agents` conventions as compatible locations. Source:
<https://opencode.ai/docs/skills/> (repo: `packages/web/src/content/docs/skills.mdx`).

Layout: one folder per skill, `SKILL.md` inside it (`<name>/SKILL.md`).

**Global (user) skill locations (all three are read):**
- `~/.config/opencode/skills/<name>/SKILL.md`  (native)
- `~/.claude/skills/<name>/SKILL.md`            (Claude-compatible)
- `~/.agents/skills/<name>/SKILL.md`            (agent-compatible)

**Project-level skill locations (all three are read):**
- `.opencode/skills/<name>/SKILL.md`
- `.claude/skills/<name>/SKILL.md`
- `.agents/skills/<name>/SKILL.md`

> Adapter note: because OpenCode reads `~/.agents/skills/*/SKILL.md` and `.agents/skills/...`
> directly, and this repo's canonical store is `~/.agents`, OpenCode may pick up dotagents
> skills with **no sync at all** for the global case. Verify this is desired vs. duplicating
> into `~/.config/opencode/skills/`.

**Frontmatter (only these fields are recognized; unknown fields ignored):**
- `name` (required) — must match the containing directory name; 1–64 chars; pattern
  `^[a-z0-9]+(-[a-z0-9]+)*$`.
- `description` (required) — 1–1024 chars.
- `license` (optional)
- `compatibility` (optional)
- `metadata` (optional, string-to-string map)

This matches the agentskills.io / Anthropic Agent Skills convention (name + description
frontmatter, directory-named skill, `SKILL.md` all-caps). Permissions for skill invocation are
configurable in `opencode.json` (allow/ask/deny).

**Symlinks:** the docs do **not** document symlinked skill directory behavior either way — treat
as UNVERIFIED. Loading is by glob (`skills/*/SKILL.md`); whether the glob resolves symlinked dirs
was not confirmed in docs and would need a live test.

---

## 2. Agents / subagents (roles)

**Supported — yes.** Two definition styles: (a) inline under the `agent` key in `opencode.json`,
or (b) per-agent markdown files. Source: <https://opencode.ai/docs/agents/>
(repo: `packages/web/src/content/docs/agents.mdx`).

**Markdown agent directories (verified plural `agents/` from repo source):**
- Global: `~/.config/opencode/agents/<name>.md`
- Project: `.opencode/agents/<name>.md`

The markdown filename becomes the agent name (`review.md` -> agent `review`). The markdown body
is the system prompt; frontmatter carries the config.

**Fields (YAML frontmatter, or JSON object under `agent.<name>`):**
- `description` (required) — what the agent is for / when to use it.
- `mode` — `primary` | `subagent` | `all`.
- `model` — override model, format `provider/model-id`
  (e.g. `anthropic/claude-sonnet-4-20250514`).
- `prompt` — path to an external system-prompt file (alternative to inline markdown body).
- `temperature` — 0.0–1.0.
- `permission` — per-tool access map, e.g. `{ edit: deny, bash: deny }` (values allow/ask/deny).
- `tools` — enable/disable specific tools.
- `steps` — max agentic iterations.
- `color` — TUI display color.
- Provider-specific params can be passed through.

Example frontmatter (from docs):
```yaml
---
description: Reviews code for quality and best practices
mode: subagent
model: anthropic/claude-sonnet-4-20250514
temperature: 0.1
permission:
  edit: deny
  bash: deny
---
```

---

## 3. MCP servers

**Declared under the `mcp` key in `opencode.json`** (global or project). Each key is the server
name. Source: <https://opencode.ai/docs/mcp-servers/>.

**Local (stdio) server:**
```json
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "my-local-mcp": {
      "type": "local",
      "command": ["npx", "-y", "@modelcontextprotocol/server-everything"],
      "environment": { "MY_VAR": "value" },
      "cwd": "/optional/working/dir",
      "enabled": true,
      "timeout": 5000
    }
  }
}
```
- `type`: `"local"` (required)
- `command`: **array of strings** (required) — NOTE: this differs from the Claude/Codex shape
  that splits `command` (string) + `args` (array). OpenCode folds the whole invocation into one
  `command` array.
- `environment`: object of env vars (NOTE: key is `environment`, not `env`).
- `cwd`, `enabled` (bool), `timeout` (ms, default 5000): optional.

**Remote server:**
```json
{
  "mcp": {
    "my-remote": {
      "type": "remote",
      "url": "https://my-mcp-server.com",
      "headers": { "Authorization": "Bearer API_KEY" },
      "enabled": true,
      "timeout": 5000
    }
  }
}
```
- `type`: `"remote"` (required), `url` (required).
- `headers` (optional), `oauth` (object or `false`), `enabled`, `timeout`.

> Adapter gotchas: (1) `command` is a single array, not command+args; (2) env key is
> `environment`; (3) MCP lives inside the shared `opencode.json`, so an adapter must
> merge into existing JSON rather than write a dedicated file.

---

## 4. Hooks / plugins

**Plugin system — yes.** Plugins are JS/TS modules; no separate "hooks" config file. Source:
<https://opencode.ai/docs/plugins/> and repo `packages/plugin/src/index.ts` (`Hooks` interface,
line ~222).

**Plugin locations (verified plural `plugins/`):**
- Global: `~/.config/opencode/plugins/*.{js,ts}`
- Project: `.opencode/plugins/*.{js,ts}`
- Or npm packages listed in config: `"plugin": ["pkg-name", ["pkg", {opts}]]`.
- External deps for local plugins: add `.opencode/package.json` (or in the config dir).
- Types: `import type { Plugin } from "@opencode-ai/plugin"`.

**Plugin shape:** an exported async function `(input, options?) => Promise<Hooks>`. The input
context provides `{ project, client, $, directory, worktree }` (`$` is a shell helper; `client`
is the OpenCode SDK client).

**Hook surface (from `Hooks` interface):**
- `event` — catch-all: receives every emitted event `{ event: Event }`.
- `config` — mutate/inspect resolved config on load.
- `auth`, `provider` — auth loaders and provider registration.
- `tool` — register custom tools; `tool.definition` to rewrite a tool's schema.
- `chat.message`, `chat.params`, `chat.headers` — intercept outgoing chat.
- `permission.ask` — approve/deny permission prompts.
- `command.execute.before`.
- `tool.execute.before`, `tool.execute.after`.
- `shell.env` — inject env for shell calls.
- `dispose` — cleanup on shutdown.
- Experimental: `experimental.chat.messages.transform`,
  `experimental.chat.system.transform`, `experimental.provider.small_model`,
  `experimental.session.compacting`, `experimental.compaction.autocontinue`,
  `experimental.text.complete`.

**Event names available via the `event` hook** (from docs "Events" list):
- Session: `session.created`, `session.updated`, `session.compacted`, `session.deleted`,
  `session.diff`, `session.error`, `session.idle`, `session.status`.
- Tool: `tool.execute.before`, `tool.execute.after`.
- Message: `message.updated`, `message.part.updated`, `message.removed`.
- File: `file.edited`, `file.watcher.updated`.
- LSP: `lsp.updated`, `lsp.client.diagnostics`.
- Command: `command.executed`. Permission: `permission.asked`.
- Server/Shell: `server.connected`, `shell.env`. Installation: `installation.updated`.
- Todo: `todo.updated`. TUI: `tui.prompt.append`, `tui.command.execute`, `tui.toast.show`.

> **Memory-integration answer:** there is **no dedicated `session-start` / `session-end` hook**
> like Claude Code's. The equivalent is a plugin subscribing via the `event` hook:
> `session.created` ~ session start; `session.idle` / `session.deleted` ~ end;
> `session.compacted` for context-compaction moments. A dotagents memory bridge would ship as a
> plugin (JS/TS) dropped in `~/.config/opencode/plugins/` that watches these events and calls out
> to the memory store. This is a code artifact, not a declarative config entry.

---

## 5. Instructions files (AGENTS.md etc.)

Source: <https://opencode.ai/docs/rules/>.
- **Reads `AGENTS.md`** — yes. Project root `AGENTS.md` applies in that dir/subdirs.
- **Global user instructions:** `~/.config/opencode/AGENTS.md`.
- **`instructions` config key** in `opencode.json`: array of extra instruction files — supports
  local paths, glob patterns (e.g. `packages/*/AGENTS.md`), and remote URLs. These combine with
  the `AGENTS.md` files.
- **Claude Code fallbacks:** project `CLAUDE.md` used if no `AGENTS.md`; `~/.claude/CLAUDE.md`
  used if no `~/.config/opencode/AGENTS.md`. Disable via `OPENCODE_DISABLE_CLAUDE_CODE=1`.
- Precedence: local `AGENTS.md` > global config instructions > Claude Code fallbacks.

---

## 6. Recent state, providers, Kimi K3

- **Version:** v1.18.3 (2026-07-16). Actively maintained, frequent releases.
- **Breaking config changes:** none surfaced in current docs for the config surfaces above; the
  `agent`/`mcp`/`plugin`/`instructions`/`skills` keys are the current stable shape. (No changelog
  diff was pulled — flag as "not exhaustively verified" if a specific version boundary matters.)
- **Providers:** OpenCode uses the Vercel AI SDK + Models.dev catalog (75+ providers, plus local
  models). Custom providers go under the `provider` key with:
  - `npm` — the AI SDK package (e.g. `@ai-sdk/openai-compatible` for any OpenAI-compatible API),
  - `options.baseURL` — custom endpoint,
  - `models` — map/array of model IDs shown in the `/models` picker.
  Source: <https://opencode.ai/docs/providers/>.
- **Kimi K3:** Moonshot released **Kimi K3 on 2026-07-16** (MoE, ~2.8T params, ~1M context).
  Moonshot's platform ships an **official "Use Kimi Models in OpenCode" guide**
  (<https://platform.kimi.ai/docs/guide/open-code>); OpenCode is listed among first-class
  supported agents. Simplest path: `opencode auth login` -> select Moonshot AI -> paste Kimi
  Open Platform API key. Custom/proxy path: define a provider with
  `npm: "@ai-sdk/openai-compatible"` + `options.baseURL`. OpenCode is a commonly-recommended CLI
  for running Kimi models (it publishes usage/rank data at `opencode.ai/data/moonshot/kimi-k3`).
  Note: the model is Kimi **K3** (K2 is the prior generation still referenced in some docs pages).

---

## Sources
- Config: <https://opencode.ai/docs/config/>
- Agents: <https://opencode.ai/docs/agents/>
- MCP: <https://opencode.ai/docs/mcp-servers/>
- Skills: <https://opencode.ai/docs/skills/>
- Plugins: <https://opencode.ai/docs/plugins/>
- Rules/instructions: <https://opencode.ai/docs/rules/>
- Providers: <https://opencode.ai/docs/providers/>
- Repo source: `github.com/sst/opencode` — `packages/web/src/content/docs/*.mdx`,
  `packages/plugin/src/index.ts`
- Release check: `gh api repos/sst/opencode/releases/latest` -> v1.18.3 (2026-07-16)
- Kimi in OpenCode: <https://platform.kimi.ai/docs/guide/open-code>,
  <https://opencode.ai/data/moonshot/kimi-k3>
