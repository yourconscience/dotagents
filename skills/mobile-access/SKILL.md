---
name: mobile-access
description: Access coding agents from iOS/Android. Covers Claude Code remote-control, Hermes Telegram gateway, terminal apps (Moshi), and web UIs.
---

# mobile-access

Use when the user wants to control a coding agent from their phone, hits remote-control disconnects, or asks about iOS/Android access to Claude Code or Hermes.

## Architecture Overview

Two fundamentally different approaches depending on where the agent runs:

| Approach | Agent location | Phone role | Stability | State access |
|---|---|---|---|---|
| Claude Code remote-control | Laptop | Remote viewer | Fragile | Chat only |
| Claude Code via Moshi/SSH | Laptop | Terminal client | Good (Mosh) | Full terminal |
| Hermes Telegram gateway | VPS | Messaging client | Best | Chat + slash commands |
| Hermes web UI | VPS | Browser client | Good | Chat + files + memory |
| Hermes CLI over SSH | VPS | Terminal client | Good | Full terminal |
| takopi (Telegram bridge) | Laptop/VPS | Messaging client | Good | Chat + files |

Server-native agents (Hermes on VPS) are inherently more stable for mobile: the agent is always on, your phone is a thin client, and reconnection is the messaging app's problem.

Laptop-native agents (Claude Code) require the laptop to stay awake and connected. KeepingYouAwake helps but does not fix the fundamental architecture.

## Claude Code: Remote Control

Built-in feature. Start from the CLI:

```bash
claude --remote-control
```

Then open Claude iOS app or visit the web URL shown.

**Known issues:**
- Sessions drop when the phone backgrounds the app (GitHub #29726)
- Silent disconnections with no auto-reconnect (#28532, #34255)
- Requires laptop to stay awake and connected
- No file browsing or memory access from phone

**Mitigations:**
- Keep the Claude app in foreground
- Use KeepingYouAwake on Mac to prevent sleep
- Refresh the page when connection drops

**Third-party alternatives:**
- **Tactic Remote** - iOS app for monitoring and approving Claude Code prompts (iOS 17+)
- **Clauder** - open-source remote access wrapper with iOS app

## Claude Code: Terminal via Moshi

Moshi is an iOS terminal app using the Mosh protocol (survives network switches, sleep, WiFi-to-cellular transitions). SSH to laptop or VPS, run `claude` in tmux.

- Push notifications on task completion
- Voice-to-terminal dictation
- Biometric auth with SSH keys in Keychain
- Proper terminal keyboard (Ctrl, Esc, arrows)

Also works for Hermes CLI sessions.

## Hermes: Telegram Gateway

The recommended mobile path for Hermes. The agent runs on a VPS; Telegram is the transport.

### 1. Create a bot

Message @BotFather on Telegram:
```
/newbot
```
BotFather returns a token: `123456789:ABCDEF1234567890abcdef...`

### 2. Get your Telegram user ID

Message @userinfobot. It replies with your numeric ID.

### 3. Configure Hermes

Add to `~/.hermes/.env`:
```bash
TELEGRAM_BOT_TOKEN=123456789:ABCDEF...
TELEGRAM_ALLOWED_USERS=<your-user-id>
```

### 4. Run the gateway

```bash
hermes gateway setup
hermes gateway install   # systemd on Linux, launchd on macOS
hermes gateway start
```

### 5. Use from iOS

Open Telegram, find your bot, start chatting. Close Telegram, reopen later, continue the same session.

Slash commands work: `/skills`, `/model`, `/memory`, `/new`, etc.

### Alternative messaging platforms

| Platform | Env vars | Notes |
|---|---|---|
| Discord | `DISCORD_BOT_TOKEN`, `DISCORD_ALLOWED_USERS` | Good if you already live in Discord |
| Slack | `SLACK_BOT_TOKEN`, `SLACK_APP_TOKEN` | Needs Socket Mode + event subscriptions |
| WhatsApp | `WHATSAPP_API_KEY`, `WHATSAPP_PHONE_NUMBER_ID` | Less stable than Telegram |

## Hermes: Web UIs

Browser-based dashboards running alongside Hermes on a VPS. Access via Tailscale for secure remote access without port forwarding. Add to iOS home screen as PWA.

- **Hermes WebUI** - lightweight, mobile-optimized SPA. Chat, sessions, skills, themes. Best mobile layout.
- **Hermes Workspace** - heavier: chat + terminal + file browser + memory browser. PWA install on iOS.

## takopi (Telegram bridge for CLI agents)

takopi bridges CLI agents to Telegram. Ships with Claude Code, Codex, OpenCode, and Pi runners. Hermes runner not built-in but the runner protocol is pluggable.

```bash
uv tool install -U takopi
takopi --onboard
```

Features: multi-project routing, worktree execution, file transfer, voice notes, session resume.

## Common Mistakes

- Do not forget `TELEGRAM_ALLOWED_USERS` on Hermes gateway. Without it, the bot accepts messages from anyone.
- Do not run `hermes gateway run` from a different machine than where your sessions live.
- Claude Code remote-control relays through Anthropic's servers; it is not the same as SSH.

## Troubleshooting

**Hermes bot does not respond:**
- `hermes gateway status`
- Verify token: `curl https://api.telegram.org/bot<TOKEN>/getMe`
- Check VPS firewall allows outbound HTTPS to `api.telegram.org`

**Claude Code remote-control disconnects:**
- Check GitHub issues #29726, #28532, #34255 for latest status
- Consider Moshi + tmux for a more stable phone experience
