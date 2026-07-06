# telegram-readonly

Read-only Telegram access for AI agents and humans via CLI + daemon.

## Architecture

```
tg CLI ──┐
MCP srv ─┤──► Unix socket ──► daemon (singleton) ──► Telegram MTProto
agent ───┘
```

A singleton daemon holds the Telethon MTProto session. All access goes
through a Unix domain socket. Multiple Claude Code sessions, Codex, Hermes,
or manual `tg` calls share one connection without auth conflicts.

The daemon auto-starts on first use and shuts down after 30 min idle.

## CLI (primary interface)

```bash
tg dialogs [--query Q] [--limit N]    # list chats
tg read <chat> [--limit N]            # read messages, batched automatically
tg download <chat> <message_id>       # download one media message
tg search <chat> <query> [--limit N]  # search in a chat
tg info <chat>                        # chat metadata
tg daemon status|start|stop|log       # manage daemon
```

Chat accepts: username, numeric ID, exact title, or title substring.

Output is JSON. Pipe through `jq` for filtering:

```bash
tg read max_akhmedov -n 10 | jq '.[] | select(.sender_name != "K K") | .text'
```

Large reads are paged through the daemon in batches of up to 100 messages,
then printed as one JSON array:

```bash
tg read max_akhmedov -n 1000 > messages.json
```

Media messages include `media_kind`, `file_name`, and `mime_type` when known.
That makes it easy to spot Telegram circles (`video_note`) and download them
as parseable video files:

```bash
tg read max_akhmedov -n 200 | jq '.[] | select(.media_kind == "video_note")'
tg download max_akhmedov 123456 --output ~/Downloads/tg/
```

## Why CLI + skill, not MCP

The MCP server is kept for backward compatibility but CLI is the primary
interface. This follows emerging best practices (mid-2025 onward):

**Lifecycle.** MCP servers are managed by the host (Claude Code spawns one
per session). This caused our original bug: 7 MCP processes fighting over
one Telethon session file. CLI delegates lifecycle to one daemon.

**Cross-agent.** CLI works from any agent with shell access (Claude Code,
Codex, Hermes, Droid, Pi). MCP requires per-agent config and not all agents
support it equally.

**Debuggability.** "When Claude does something unexpected, I can run the
same command and see exactly what it saw" (HN, Jun 2026). CLI is transparent;
MCP is a black box to the user.

**Composability.** Unix pipes, jq, grep, redirection. MCP has no equivalent.

**Token cost (nuanced).** Modern hosts like Claude Code lazily load MCP
tool schemas (ToolSearch), so the classic "55K tokens per MCP" bloat no
longer applies. But CLI still costs zero schema tokens and agents already
know shell patterns from training data.

**When MCP still wins:** OAuth flows, non-technical users, enterprise audit
trails, browser-only environments, stateful multi-step protocols. None of
these apply to a read-only Telegram client.

Decision rule: if the tool has a CLI and the user has shell access, prefer
CLI + skill over MCP. Reserve MCP for services that only expose an API,
need per-request auth, or have no CLI equivalent.

Sources:
- Simon Willison, "Too many MCPs" (Aug 2025)
- Simon Willison, "Claude Skills are awesome, maybe a bigger deal than MCP" (Oct 2025)
- HN discussion, "When does MCP make sense vs CLI?" (Jun 2026)
- Firecrawl, "MCP vs CLI for AI Agents" (2026)

## Skill

The `/tg` skill (`~/.agents/skills/tg/SKILL.md`) teaches agents how to use
the CLI. It is the recommended way to give any agent Telegram read access.

## Setup

1. Get API credentials from https://my.telegram.org (API development tools).

2. Create `.env`:
   ```bash
   cd ~/.agents/mcp/telegram-readonly
   cp .env.example .env
   # fill TELEGRAM_API_ID and TELEGRAM_API_HASH
   ```

3. One-time login (interactive, do not paste secrets into agent chat):
   ```bash
   cd ~/.agents/mcp/telegram-readonly && uv run python login.py
   ```

4. Verify:
   ```bash
   tg dialogs -n 3
   ```

## Files

| Path | Purpose |
|------|---------|
| `server.py` | Daemon + MCP proxy server |
| `tg` | CLI (symlinked to `~/.local/bin/tg`) |
| `login.py` | Interactive Telegram login |
| `~/.local/share/dotagents/telegram-readonly/` | Session, socket, lock, logs |

## Security

- The `.session` file is a user credential. Protect it.
- Read-only by construction: no send/edit/delete tools exist.
- Daemon socket is `chmod 600`.
- Message bodies are returned only via explicit tool/CLI calls, not logged.
