# Telegram read-only MCP

Local MTProto MCP server for reading Telegram dialogs and messages from agent clients that support MCP.

Exposed tools:
- `list_dialogs`
- `get_chat_info`
- `get_recent_messages`
- `search_messages`

The server intentionally does not implement send, edit, or delete tools.

## Dotagents integration

The canonical MCP entry lives in `skills/dotagents/dotagents.yaml` as `telegram_readonly` and targets:
- Amp
- Claude Code
- Codex
- Hermes
- Factory Droid

`dotagents sync` writes each agent's native MCP config. The managed command runs the repo copy through `~/.agents`:

`sh -lc 'exec uv run --project ~/.agents/mcp/telegram-readonly python ~/.agents/mcp/telegram-readonly/server.py'`

The shell wrapper is deliberate: most MCP clients execute commands directly and do not expand `~` inside argument arrays.

## Local setup

Run this on each machine where the MCP server should access your Telegram account.

1. Get Telegram API credentials from `https://my.telegram.org` -> API development tools.

2. Create the local env file:

`cd ~/.agents/mcp/telegram-readonly`
`cp .env.example .env`
`$EDITOR .env`

Fill:

`TELEGRAM_API_ID=...`
`TELEGRAM_API_HASH=...`

Do not paste these secrets into agent chat.

3. One-time login from a real terminal:

`cd ~/.agents/mcp/telegram-readonly`
`uv run python login.py`

This may ask for phone, Telegram code, and 2FA password. Type them yourself.

4. Sync/restart the agent client:

`cd ~/.agents`
`go run ./skills/dotagents/tools/dotagents sync --agents=hermes`
`hermes gateway restart`

For other agents, run `dotagents sync` for that agent and restart/reload the client so it rediscovers MCP servers.

## Runtime state

Default session file:

`~/.local/share/dotagents/telegram-readonly/telegram.session`

Override with `TELEGRAM_SESSION_PATH` in `.env` if needed.

## Example prompts

- `list my recent Telegram dialogs`
- `search my Telegram chat with Victor for Paris`
- `read recent messages from Telegram chat Natasha`

## Security notes

- The Telegram API session is a user session, so protect the `.session` file like a secret.
- The MCP layer is read-only by construction: no write tools are registered.
- Message bodies are returned only through explicit tool calls and are not logged by the server.
