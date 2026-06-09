---
name: tg
description: Read Telegram chats, search messages, and list dialogs via the `tg` CLI. Use when the user asks to check Telegram, read a chat, find a message, or monitor for replies.
---

# tg -- Read-only Telegram CLI

Use this skill to read Telegram messages. All operations are read-only.

## Commands

```bash
tg dialogs                       # list recent chats (default 20)
tg dialogs --query "Berlin"      # filter by name substring
tg dialogs --limit 50            # more results

tg read <chat> [--limit N]       # read recent messages
tg search <chat> <query> [-n N]  # search within a chat
tg info <chat>                   # chat metadata

tg daemon status                 # check if daemon is running
tg daemon start                  # start daemon manually
tg daemon stop                   # stop daemon
tg daemon log                    # show daemon log tail
```

## Chat resolution

The `<chat>` argument accepts any of:
- Username: `max_akhmedov`, `wq67753`
- Numeric ID: `118488548`
- Exact chat title: `"Кирилл Кориков - Берлин"`
- Title substring: `Берлин` (matches first dialog containing it)

## Output

All commands output JSON. Pipe through `jq` for filtering:

```bash
tg read wq67753 -n 5 | jq '.[] | select(.sender_name == "Модник") | .text'
```

## Architecture

A singleton daemon holds the Telethon MTProto session. The CLI and MCP servers proxy through a Unix domain socket. Multiple Claude Code sessions can read Telegram simultaneously without auth conflicts.

The daemon auto-starts on first use and shuts down after 30 min idle.

## Session management

If the session expires or gets corrupted:
```bash
tg daemon stop
cd ~/.agents/mcp/telegram-readonly && uv run python login.py
tg daemon start
```

## Monitoring pattern

Check for new messages from a specific person:
```bash
tg read <chat> -n 5 | jq '[.[] | select(.sender_name == "Name" and .date > "2026-06-09T00:00:00")] | length'
```
