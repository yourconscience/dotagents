---
name: remote-access
description: Use mobile-friendly remote coding access, preferring Untether for multi-agent Telegram control and the Mac bridge for read-only local Droid/Codex inspection from Hermes.
---

# remote-access

Use this when the user is on mobile and wants to inspect, continue, or control coding agents remotely.

Prefer **Untether** when the request is to run or control agents from a phone. Untether is the first-class mobile control plane for provider-agnostic agent work because it bridges Telegram to Claude Code, Codex, OpenCode, Pi, Gemini CLI, and Amp.

Use the **Mac bridge** path when Hermes on the VPS needs to inspect or lightly continue existing local Mac Droid/Codex work without exposing a terminal.

Do not use the Mac bridge for session transfer, live terminal sharing, tmux, Warp/TUI scraping, or unrestricted shell access. For live phone control, use Untether, mobile SSH, or a web terminal instead of extending the Mac bridge beyond its narrow inspection/ask API.

## Choose the path

| Need | Use | Notes |
|---|---|---|
| Start/control agents from phone | Untether | Best default for mobile. Telegram text/voice, progress streaming, approvals, `/health`, `/stats`, `/continue`. |
| Provider-agnostic mobile workflow | Untether | Supports Claude Code, Codex, OpenCode, Pi, Gemini CLI, and Amp. |
| Inspect local Mac sessions from Hermes/VPS | Mac bridge | Read-only status/search plus scoped Droid continuation instructions. |
| Full live terminal on phone | Mobile SSH / ttyd / WeTTY | Keep tmux as truth; do not use the Mac bridge for this. |
| Desktop visual cockpit | cmux / tmux | Useful locally; not the preferred phone access layer. |

## Untether mobile control plane

Untether is appropriate when the user asks for `/mobile-access`, phone control, Telegram control, Takopi-style workflows, or multi-agent mobile access.

Install or update:

```bash
uv tool install untether
uv tool upgrade untether
```

Initial setup:

```bash
untether --onboard
```

Register a project from the repo root:

```bash
untether init <alias>
untether init <alias> --default
```

Important hardening before using it for real work:

```toml
[transports.telegram]
allowed_user_ids = [123456789]

[engines.amp]
dangerously_allow_all = false
```

Use `untether chat-id` to capture the Telegram user/chat IDs. Protect the config because it contains the Telegram bot token:

```bash
chmod 600 ~/.untether/untether.toml
```

Run diagnostics:

```bash
untether doctor
untether plugins --load
```

Useful Telegram commands:

```text
/agent              show or set engine
/claude ...         run Claude Code
/codex ...          run Codex
/amp ...            run Amp
/continue ...       resume latest local CLI session where supported
/health             process and trigger health
/stats              per-engine run counts and durations
/usage              usage/cost where supported
/cancel             stop current run
/config             inline settings
```

Security notes:

- Empty `allowed_user_ids` means anyone who discovers the bot can start runs.
- Untether runs engine CLIs on the host; project allowlists and conservative engine approval settings matter.
- Claude/Pi have subprocess environment allowlisting in Untether v0.35.2; Codex/Gemini/OpenCode/Amp may inherit more parent environment, so do not run it from a shell containing broad secrets.
- Amp support is useful but should not default to all-access mode.

Untether complements cmux/tmux rather than replacing them: use cmux/tmux as the local cockpit and Untether as the phone-native control plane.

## Mac bridge inspection path

## What it does

- Checks whether the Mac bridge is online.
- Lists recent local Droid/Codex sessions.
- Searches past session history by query or session ID.
- Reports repo, branch, git status, changed files, mission state, and likely next action.
- Sends small, auditable instructions to an existing Droid session when explicitly requested.

## Bridge endpoints

```text
Mac local: http://127.0.0.1:18777
VPS to Mac over Tailscale: $REMOTE_ACCESS_BRIDGE_URL
```

Hermes on the VPS should call the Mac bridge directly over Tailscale. Set `REMOTE_ACCESS_BRIDGE_URL` to the Mac bridge URL, for example `http://<mac-tailscale-host>:18777`. Prefer binding the bridge to the Mac's Tailscale address instead of all interfaces.

## Commands

Status is unauthenticated:

```bash
curl -fsS "$REMOTE_ACCESS_BRIDGE_URL/status"
```

Recent/search/ask require `REMOTE_ACCESS_BRIDGE_TOKEN`:

```bash
curl -fsS -H "authorization: Bearer $REMOTE_ACCESS_BRIDGE_TOKEN" \
  "$REMOTE_ACCESS_BRIDGE_URL/recent"

curl -fsS -H "authorization: Bearer $REMOTE_ACCESS_BRIDGE_TOKEN" \
  --get --data-urlencode 'q=<query>' \
  "$REMOTE_ACCESS_BRIDGE_URL/search"

curl -fsS -X POST "$REMOTE_ACCESS_BRIDGE_URL/ask" \
  -H 'content-type: application/json' \
  -H "authorization: Bearer $REMOTE_ACCESS_BRIDGE_TOKEN" \
  -d '{"agent":"droid","session_id":"<id>","instruction":"summarize current state and next action","mode":"continue"}'
```

`X-Remote-Access-Token: <token>` is also accepted.

## Procedure

1. Run `/status` first and say whether the Mac is online.
2. For unclear requests, run `/recent` or `/search` before `/ask`.
3. Prefer read-only summaries. Use `/ask` only when the user gives a target session or explicitly asks to continue one.
4. Keep `/ask` instructions narrow: session ID, repo/path, expected output, allowed file writes, commit policy, and files not to touch.
5. If the bridge is offline, use synced knowledge-vault context if available and label it stale.

## Response shape

For mobile, keep output compact:

```text
Mac: online/offline
Repo: <path>
Branch: <branch>
Git: <status summary>
Session: <id/title>
State: <mission/session state>
Blockers: <none/list>
Next action: <single command or instruction>
```

## Local data sources

- Droid: `droid search --json`, `~/.factory/sessions/**`, `droid exec --session-id`, `droid exec --fork`
- Codex: `~/.codex` history/session stores and local continuation commands when available
- Git: `git status --short --branch`, `git diff --stat`, recent commits
- Cached fallback: synced knowledge vault at `$KNOWLEDGE_DIR`

## Bridge maintenance

Build or refresh the Mac runtime binary:

```bash
~/.agents/skills/remote-access/tools/remote-access-bridge/build.sh
launchctl kickstart -k "gui/$(id -u)/com.conscience.remote-access.bridge"
```

The binary defaults to `~/.local/bin/remote-access-bridge`.

If VPS access fails, check Tailscale connectivity first:

```bash
ssh vps-ts 'curl -fsS --max-time 5 "$REMOTE_ACCESS_BRIDGE_URL/status"'
```
