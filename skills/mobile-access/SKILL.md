---
name: mobile-access
description: Access coding agents from iOS/Android. Covers Tailscale-based remote access, messaging gateways, terminal apps (Moshi), and web UIs. Agent-agnostic with per-agent specifics at the end.
---

# mobile-access

Use when the user wants to control a coding agent from their phone, needs remote access to a development machine, or asks about iOS/Android access to Claude Code, Hermes, Codex, or similar agents.

## Architecture Overview

Two fundamentally different approaches depending on where the agent runs:

| Approach | Agent location | Phone role | Stability | State access |
|---|---|---|---|---|
| Tailscale + Web UI | Local machine | Browser client | Best | Chat + files + memory |
| Tailscale + SSH/Moshi | Local machine | Terminal client | Best | Full terminal |
| Messaging gateway | Local or server | Messaging client | Best | Chat + slash commands |
| Agent remote-control | Local machine | Remote viewer | Fragile | Chat only |

**Preferred path for non-chat tasks:** Tailscale + web UI or Tailscale + Moshi. Your phone connects directly to your local machine over the tailnet. No VPS, no port forwarding, no public exposure.

**Preferred path for chat tasks:** Messaging gateway (Telegram, Discord, Slack). The agent is always available; the messaging app handles reconnection.

## Tailscale Setup

Tailscale creates a private WireGuard mesh network. Once set up, your phone can reach your Mac directly as if on the same LAN.

### Install on Mac

```bash
brew install --cask tailscale-app
sudo ln -sf /Applications/Tailscale.app/Contents/MacOS/Tailscale /usr/local/bin/tailscale
```

Open Tailscale.app and sign in.

### Verify

```bash
tailscale status
tailscale ip -4
```

### Connect other machines

```bash
# Linux VPS
curl -fsSL https://tailscale.com/install.sh | sh
tailscale up
```

Each device gets a `100.x.x.x` IP reachable only from your tailnet.

### Enable Tailscale Serve (for web UIs)

Tailscale Serve proxies a local HTTP service to HTTPS on your tailnet hostname:

```bash
tailscale serve --bg --https=443 http://127.0.0.1:PORT
tailscale serve status
```

This gives you `https://your-machine.tailnet.ts.net/` accessible from any device on the tailnet, including your phone.

To disable:
```bash
tailscale serve --https=443 off
```

## Phone Access via Tailscale

### iOS Setup

1. Install **Tailscale** from the App Store
2. Sign in with the same account as your Mac
3. Your Mac is now reachable at its Tailscale IP or MagicDNS hostname

### Non-Chat Access: Web UI (recommended)

If the agent has a web dashboard exposed via Tailscale Serve:

1. Open Safari on your phone
2. Navigate to `https://your-machine.tailnet.ts.net/`
3. Tailscale handles the HTTPS cert automatically
4. Add to Home Screen for PWA behavior

**No extra work needed on the phone** beyond having Tailscale installed and signed in.

### Non-Chat Access: SSH via Moshi

For full terminal access:

1. Install **Moshi** from the App Store (iOS terminal using Mosh protocol)
2. Add your Mac's Tailscale IP as a host
3. Set up SSH key auth (Moshi stores keys in iOS Keychain with biometric unlock)
4. SSH in, run the agent in tmux

Moshi survives network switches, sleep, and WiFi-to-cellular transitions. Push notifications on task completion. Voice-to-terminal dictation.

### Non-Chat Access: Tailscale SSH

Tailscale has built-in SSH (no port forwarding, key management, or password needed):

```bash
# On Mac, enable Tailscale SSH
tailscale up --ssh
```

Then from any Tailscale device (including phone via terminal app):
```bash
ssh conscience@kirills-macbook-pro
```

### Chat Access: Messaging Gateway

For conversational use, a messaging gateway is better than web UI:
- Telegram, Discord, Slack, WhatsApp, etc.
- The messaging app handles reconnection, notifications, media
- No Tailscale needed (gateway can be on VPS or local)

See per-agent sections below for gateway setup.

## Third-Party Tools

- **takopi** — Telegram bridge for CLI agents. Ships with Claude Code, Codex, OpenCode runners. `uv tool install -U takopi && takopi --onboard`
- **Tactic Remote** — iOS app for monitoring and approving Claude Code prompts (iOS 17+)
- **Clauder** — open-source remote access wrapper with iOS app

## Common Mistakes

- Do not forget `TELEGRAM_ALLOWED_USERS` on messaging gateways. Without it, the bot accepts messages from anyone.
- Do not expose services to `0.0.0.0` on public networks. Use `tailscale serve` instead of `--insecure` binds. On a tailnet, `--insecure` is fine since the network is private.
- Tailscale Serve requires the Serve feature to be enabled on your tailnet first. It will prompt you with a URL to visit.
- When using Tailscale Serve, bind the local service to `0.0.0.0` (not `127.0.0.1`) so it accepts the MagicDNS hostname in the Host header. `hermes dashboard` validates Host headers and rejects requests to `127.0.0.1` when the Host is `*.ts.net`.
- `brew install --cask tailscale-app` does not put the CLI on PATH. Create a symlink: `sudo ln -sf /Applications/Tailscale.app/Contents/MacOS/Tailscale /usr/local/bin/tailscale`
- Claude Code remote-control relays through Anthropic's servers; it is not the same as SSH.

## Troubleshooting

**Tailscale devices can't reach each other:**
- `tailscale status` — both devices should show as connected
- Check if Tailscale is running (menu bar icon on Mac)
- `tailscale ping <ip>` — verify direct connectivity

**Web UI "Invalid Host header" error:**
- The dashboard is bound to `127.0.0.1` but Tailscale Serve sends `*.ts.net` as Host
- Fix: restart dashboard with `--host 0.0.0.0`
- Or skip Tailscale Serve and access via `http://100.x.x.x:9119` directly

**Web UI not accessible from phone:**
- `tailscale serve status` — verify serve is running
- Try `tailscale ip -4` and access by IP first
- Check firewall on Mac allows Tailscale interface

**Moshi disconnects:**
- Ensure Mac doesn't sleep (KeepingYouAwake or `caffeinate`)
- Check SSH is running: `sudo systemsetup -getremotelogin`

---

## Per-Agent Notes

### Hermes

**Current preferred setup (Mac-hosted):**
- Gateway on Mac with Telegram
- Dashboard on port 9119, exposed via Tailscale Serve
- URL: `https://kirills-macbook-pro.taila0218e.ts.net/`

**Quick commands:**
```bash
hermes gateway start        # start Telegram gateway
hermes gateway status       # check status
hermes dashboard --no-open  # start web UI on :9119
```

**Dashboard setup (Tailscale Serve):**
```bash
# IMPORTANT: bind to 0.0.0.0, not 127.0.0.1
# Tailscale Serve sends the MagicDNS hostname as Host header,
# which hermes dashboard rejects when bound to 127.0.0.1
hermes dashboard --no-open --host 0.0.0.0 --port 9119 --insecure
tailscale serve --bg --https=443 http://127.0.0.1:9119
```

**Dashboard setup (direct IP, no Tailscale Serve):**
```bash
hermes dashboard --no-open --host 0.0.0.0 --port 9119 --insecure
# Access via http://100.x.x.x:9119 from any tailnet device
# Shorter URL but no HTTPS
```

**Mobile access:**
- Chat: Telegram bot (gateway handles reconnection)
- Web: Tailscale Serve URL (dashboard, sessions, skills, memory)
- Terminal: Moshi + tmux, or Tailscale SSH

**Config:** `~/.hermes/config.yaml`, `~/.hermes/.env`

### Claude Code

**Remote control:**
```bash
claude --remote-control
```
Open Claude iOS app or web URL. Fragile — sessions drop on background.

**Terminal via Moshi (more stable):**
- SSH to Mac, run `claude` in tmux
- Survives network switches and sleep

**Third-party:**
- Tactic Remote (iOS) for monitoring/approval
- takopi for Telegram bridge

### OpenAI Codex

- CLI agent, no built-in web UI
- Access via Tailscale + SSH/Moshi
- Or takopi for Telegram bridge
- Subscription auth can be flaky from VPS IPs (Cloudflare challenges); prefer local Mac

### OpenClaw / OpenCode

- CLI agents, same access pattern as Codex
- Tailscale + SSH/Moshi or takopi

### General CLI Agents

For any terminal-based agent:
1. Tailscale + SSH/Moshi for interactive terminal
2. takopi for Telegram bridge
3. tmux for session persistence
