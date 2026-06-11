---
name: remote-access
description: Search local Droid/Codex sessions, send scoped continuation instructions through the Mac bridge, or use takopi for Telegram-based mobile access to Pi sessions.
---


# remote-access

Two modes of mobile access:

1. **takopi** (recommended for Pi) — Telegram bridge that streams progress, supports voice input, project/worktree management, and session resume. Use when the user wants to send prompts to Pi from their phone and monitor agent work.
2. **Mac bridge** — lightweight REST bridge for Hermes on VPS to inspect/continue local Droid/Codex sessions. Use when the user wants Hermes to peek at or lightly continue Mac-local agent work.

Do not use this for session transfer, live terminal sharing, tmux, Warp/TUI scraping, or unrestricted shell access.

## takopi (Telegram → Pi)

[takopi](https://github.com/banteg/takopi) bridges Pi to a Telegram bot. Install: `uv tool install -U takopi`. Docs: [takopi.dev](https://takopi.dev/).

The user's bot is `@gamrevinu_bot`. Config lives at `~/.takopi/takopi.toml`.

### Quick reference

```bash
takopi                    # setup wizard (first run) or start bridge
takopi config list        # show current config
takopi config set default_engine pi
takopi config set pi.provider anthropic
```

From Telegram, prefix messages with `/pi` to target Pi, or set `default_engine = "pi"`.

### Capabilities with Pi

- Progress streaming (tool calls, file changes, elapsed time)
- Session resume (`pi --session <token>`)
- Voice notes (transcribed via Whisper endpoint)
- Project aliases and git worktrees
- File transfer (`/file put`, `/file get`)

### Limitations

takopi uses `pi --print --mode json` (non-interactive). No mid-stream steering, tool approval, model switching, or compaction. For full RPC control, wait for `pi serve` (planned).


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

Build or refresh the Mac runtime binary (`$SKILL_DIR` = this skill's own directory):

```bash
"$SKILL_DIR"/tools/remote-access-bridge/build.sh
launchctl kickstart -k "gui/$(id -u)/com.conscience.remote-access.bridge"
```

The binary defaults to `~/.local/bin/remote-access-bridge`.

If VPS access fails, check Tailscale connectivity first:

```bash
ssh vps-ts 'curl -fsS --max-time 5 "$REMOTE_ACCESS_BRIDGE_URL/status"'
```
## Future: pi serve

`pi serve` will expose the full RPC protocol over HTTP/WebSocket, enabling:
- Full interactive control from any device (steer, abort, model switch, compaction)
- Web UI for browser-based access
- Richer takopi integration via WebSocket transport backend

This is tracked in [oh-my-pi#436](https://github.com/can1357/oh-my-pi/issues/436).

