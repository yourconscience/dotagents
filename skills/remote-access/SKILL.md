---
name: remote-access
description: Search local Droid/Codex sessions and send scoped continuation instructions through the Mac bridge when using Hermes from mobile.
---

# remote-access

Use this when the user is on mobile and wants Hermes on the VPS to inspect or lightly continue local Mac agent work.

Do not use this for session transfer, live terminal sharing, tmux, Warp/TUI scraping, or unrestricted shell access.

## What it does

- Checks whether the Mac bridge is online.
- Lists recent local Droid/Codex sessions.
- Searches past session history by query or session ID.
- Reports repo, branch, git status, changed files, mission state, and likely next action.
- Sends small, auditable instructions to an existing Droid session when explicitly requested.

## Bridge endpoints

```text
Mac local: http://127.0.0.1:18777
VPS to Mac over Tailscale: http://100.80.21.89:18777
```

Hermes on the VPS should call the Mac bridge directly over Tailscale. The Mac bridge LaunchAgent must listen on `0.0.0.0:18777` or `*:18777`.

## Commands

Status is unauthenticated:

```bash
curl -fsS http://100.80.21.89:18777/status
```

Recent/search/ask require `REMOTE_ACCESS_BRIDGE_TOKEN`:

```bash
curl -fsS -H "authorization: Bearer $REMOTE_ACCESS_BRIDGE_TOKEN" \
  http://100.80.21.89:18777/recent

curl -fsS -H "authorization: Bearer $REMOTE_ACCESS_BRIDGE_TOKEN" \
  --get --data-urlencode 'q=<query>' \
  http://100.80.21.89:18777/search

curl -fsS -X POST http://100.80.21.89:18777/ask \
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
- Cached fallback: synced knowledge vault at `~/Workspace/knowledge`

## Bridge maintenance

Build or refresh the Mac runtime binary:

```bash
~/.agents/skills/remote-access/tools/remote-access-bridge/build.sh
launchctl kickstart -k "gui/$(id -u)/com.conscience.remote-access.bridge"
```

The binary defaults to `~/.local/bin/remote-access-bridge`.

If VPS access fails, check Tailscale connectivity first:

```bash
ssh vps-ts 'curl -fsS --max-time 5 http://100.80.21.89:18777/status'
```
