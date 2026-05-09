---
name: handoff
description: Access local Droid/Codex work from mobile through Hermes on the VPS. Use for handoff status, recent sessions, search, and simple continuation.
---

# handoff

Use this skill when the user wants to inspect, continue, or route local coding-agent work from a phone.

## Definition: handoff

Handoff is **state transfer plus an optional command path**, not live terminal mirroring.

A handoff packet answers:

- What was I doing?
- In which repo and branch?
- Which Droid/Codex sessions are relevant?
- What changed locally?
- Is the Mac online for live continuation?
- What command can safely continue the work?

## V1 architecture

Two layers only:

1. **Offline read layer:** knowledge vault sync.
   - Mac writes durable summaries to `~/Workspace/knowledge`.
   - Knowledge repo syncs to the VPS bare remote.
   - Hermes on the VPS can answer from cached context when the Mac is offline.
   - This is read-only unless Hermes starts a new VPS-side agent from cached context.

2. **Online live layer:** Mac handoff bridge.
   - A macOS `launchd` job keeps a small local bridge running.
   - The bridge maintains a persistent reverse SSH tunnel to the VPS.
   - Hermes calls the bridge through `http://127.0.0.1:<vps-port>` on the VPS.
   - The bridge answers local `recent`, `search`, `status`, and `ask` requests without per-request SSH startup.

```text
iPhone Telegram
  -> Hermes gateway on VPS
    -> cached knowledge vault when Mac is offline
    -> localhost bridge endpoint on VPS when Mac is online
      -> reverse SSH tunnel
        -> Mac handoff bridge
          -> Droid/Codex session stores + git state + safe continuation commands
```

## Transport choice

V1 uses a **reverse SSH tunnel plus local HTTP bridge**.

Preferred shape:

```text
Mac:
  handoff bridge listens on 127.0.0.1:<mac-port>
  launchd keeps:
    ssh -N -R 127.0.0.1:<vps-port>:127.0.0.1:<mac-port> vps

VPS:
  Hermes calls:
    http://127.0.0.1:<vps-port>/status
    http://127.0.0.1:<vps-port>/recent
    http://127.0.0.1:<vps-port>/search?q=...
    http://127.0.0.1:<vps-port>/ask
```

Reasons:

- no phone Tailscale dependency
- no Termius workflow
- no inbound Mac ports
- encrypted with existing SSH
- fast after startup
- easier to debug than a custom WebSocket service

## Data sources

### Droid

- `droid search --json`
- `~/.factory/sessions/**`
- `droid exec --session-id <id> <prompt>`
- `droid exec --fork <id> <prompt>`

### Codex

- `~/.codex` session/history stores
- `codex` / `omx` continuation commands when available
- repo-local git state

### Git/repo state

For each detected active repo:

- `pwd`
- `git status --short --branch`
- `git diff --stat`
- recent branch/commit metadata

Do not rely on Warp screen scraping. Warp/TUI state is not a stable API.

## Telegram/Hermes commands

Target commands:

```text
/handoff status
/handoff recent
/handoff search <query>
/handoff context <session-or-repo>
/handoff ask <session-id> <instruction>
/handoff fork <session-id> <instruction>
```

When Mac is offline:

- answer from synced knowledge
- say the Mac is unavailable
- offer to start a new VPS-side agent only from cached context

When Mac is online:

- query the bridge for fresh local context
- continue Droid/Codex through explicit session commands
- keep commands scoped and auditable

## Non-goals for V1

- no live terminal screen takeover
- no tmux requirement
- no Termius-first workflow
- no Hermes gateway on Mac as the primary path
- no custom WebSocket protocol unless reverse SSH is proven insufficient
- no raw secret/env/session dump into Telegram

## Implementation order

1. Verify knowledge sync health on Mac and VPS.
2. Run the Mac bridge on `127.0.0.1:18777`.
3. Expose it to the VPS with a reverse SSH tunnel on `127.0.0.1:18778`.
4. Configure Hermes to call `http://127.0.0.1:18778`.
5. Use `/ask` for small Droid follow-ups only after checking `/status`, `/recent`, or `/search`.
6. Add Codex continuation after Droid continuation is working.

## Installed local endpoints

The V1 bridge is expected at:

```text
Mac local: http://127.0.0.1:18777
VPS local: http://127.0.0.1:18778
```

Useful commands from the VPS:

```bash
curl -fsS http://127.0.0.1:18778/status
curl -fsS -H "authorization: Bearer $HANDOFF_BRIDGE_TOKEN" \
  http://127.0.0.1:18778/recent
curl -fsS -H "authorization: Bearer $HANDOFF_BRIDGE_TOKEN" \
  --get --data-urlencode 'q=handoff' http://127.0.0.1:18778/search
curl -fsS -X POST http://127.0.0.1:18778/ask \
  -H 'content-type: application/json' \
  -H "authorization: Bearer $HANDOFF_BRIDGE_TOKEN" \
  -d '{"agent":"droid","session_id":"<id>","instruction":"summarize current state","mode":"continue"}'
```

`/recent`, `/search`, and `/ask` are disabled unless the Mac bridge process has
`HANDOFF_BRIDGE_TOKEN` set. Send the same value as `Authorization: Bearer ...`
or `X-Handoff-Token`.

## Build and install bridge

The bridge binary is not tracked in git. Build or refresh the local runtime
binary from source with:

```bash
~/.agents/skills/handoff/tools/handoff-bridge/build.sh
launchctl kickstart -k "gui/$(id -u)/com.conscience.handoff.bridge"
```
