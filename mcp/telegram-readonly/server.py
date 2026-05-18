#!/usr/bin/env python3
"""Read-only Telegram MCP server.

Security model:
- Uses the user's MTProto session file locally.
- Exposes only read/list/search tools. No send/delete/edit methods exist.
- Does not log message bodies.
"""
from __future__ import annotations

import asyncio
import os
import re
import shlex
from datetime import timezone
from pathlib import Path
from typing import Any

from mcp.server.fastmcp import FastMCP
from telethon import TelegramClient
from telethon.utils import get_peer_id
from telethon.tl.types import User, Chat, Channel

ROOT = Path(__file__).resolve().parent
ENV_PATH = Path(os.getenv("TELEGRAM_READONLY_ENV", str(ROOT / ".env"))).expanduser()
ENV_KEY_RE = re.compile(r"^[A-Za-z_][A-Za-z0-9_]*$")

mcp = FastMCP("telegram-readonly")
_client: TelegramClient | None = None
_lock = asyncio.Lock()


def _strip_unquoted_comment(value: str) -> str:
    quote = ""
    escaped = False
    for i, char in enumerate(value):
        if escaped:
            escaped = False
            continue
        if quote and char == "\\":
            escaped = True
            continue
        if char in "'\"" and not quote:
            quote = char
            continue
        if char == quote:
            quote = ""
            continue
        if char == "#" and not quote and (i == 0 or value[i - 1].isspace()):
            return value[:i]
    return value


def _parse_env_line(line: str) -> tuple[str, str] | None:
    line = line.strip()
    if not line or line.startswith("#"):
        return None
    if line.startswith("export "):
        line = line[len("export ") :].lstrip()
    if "=" not in line:
        return None
    key, value = line.split("=", 1)
    key = key.strip()
    if not ENV_KEY_RE.fullmatch(key):
        return None
    parts = shlex.split(_strip_unquoted_comment(value), comments=False, posix=True)
    return key, " ".join(parts)


def _load_env() -> None:
    if ENV_PATH.exists():
        for line_no, line in enumerate(ENV_PATH.read_text().splitlines(), 1):
            try:
                parsed = _parse_env_line(line)
            except ValueError as err:
                raise RuntimeError(f"Invalid .env line {line_no} in {ENV_PATH}: {err}") from err
            if parsed is None:
                continue
            key, value = parsed
            os.environ.setdefault(key, value)


def _require_env() -> tuple[int, str, str]:
    _load_env()
    api_id = os.getenv("TELEGRAM_API_ID")
    api_hash = os.getenv("TELEGRAM_API_HASH")
    session_path = os.getenv(
        "TELEGRAM_SESSION_PATH",
        str(Path.home() / ".local" / "share" / "dotagents" / "telegram-readonly" / "telegram.session"),
    )
    if not api_id or not api_hash:
        raise RuntimeError(f"TELEGRAM_API_ID/TELEGRAM_API_HASH missing. Fill {ENV_PATH}")
    return int(api_id), api_hash, str(Path(session_path).expanduser())


async def _get_client() -> TelegramClient:
    global _client
    async with _lock:
        if _client is not None and _client.is_connected():
            if await _client.is_user_authorized():
                return _client
            await _client.disconnect()
            _client = None
        api_id, api_hash, session_path = _require_env()
        Path(session_path).parent.mkdir(parents=True, exist_ok=True)
        client = TelegramClient(session_path, api_id, api_hash)
        await client.connect()
        if not await client.is_user_authorized():
            await client.disconnect()
            _client = None
            raise RuntimeError(f"Telegram session is not authorized. Run: cd {ROOT} && uv run python login.py")
        _client = client
        return client


def _entity_name(entity: Any) -> str:
    if isinstance(entity, User):
        parts = [entity.first_name or "", entity.last_name or ""]
        name = " ".join(p for p in parts if p).strip()
        return name or entity.username or str(entity.id)
    return getattr(entity, "title", None) or getattr(entity, "username", None) or str(getattr(entity, "id", "unknown"))


def _entity_kind(entity: Any) -> str:
    if isinstance(entity, User):
        return "user" if not entity.bot else "bot"
    if isinstance(entity, Channel):
        if getattr(entity, "broadcast", False):
            return "channel"
        if getattr(entity, "megagroup", False):
            return "megagroup"
        return "channel"
    if isinstance(entity, Chat):
        return "group"
    return type(entity).__name__


def _message_to_dict(msg: Any) -> dict[str, Any]:
    sender = getattr(msg, "sender", None)
    return {
        "id": msg.id,
        "date": msg.date.astimezone(timezone.utc).isoformat() if msg.date else None,
        "sender_id": getattr(msg, "sender_id", None),
        "sender_name": _entity_name(sender) if sender else None,
        "text": msg.message or "",
        "has_media": bool(getattr(msg, "media", None)),
        "reply_to_msg_id": getattr(getattr(msg, "reply_to", None), "reply_to_msg_id", None),
    }


async def _resolve_chat(chat: str) -> Any:
    client = await _get_client()
    chat = str(chat).strip()
    if not chat:
        raise ValueError("chat is required: username, numeric id, exact title, or substring title")
    if re.fullmatch(r"-?\d+", chat):
        try:
            return await client.get_entity(int(chat))
        except Exception:
            pass
    try:
        return await client.get_entity(chat)
    except Exception:
        pass
    low = chat.lower()
    async for dialog in client.iter_dialogs(limit=500):
        if dialog.name and (dialog.name.lower() == low or low in dialog.name.lower()):
            return dialog.entity
    raise ValueError(f"Could not resolve Telegram chat: {chat!r}")


@mcp.tool()
async def list_dialogs(limit: int = 50, query: str | None = None) -> list[dict[str, Any]]:
    """List recent Telegram dialogs. Optional case-insensitive title/name filter."""
    client = await _get_client()
    out: list[dict[str, Any]] = []
    q = query.lower() if query else None
    async for dialog in client.iter_dialogs(limit=max(1, min(limit, 200))):
        if q and q not in (dialog.name or "").lower():
            continue
        entity = dialog.entity
        out.append({
            "id": dialog.id,
            "raw_id": getattr(entity, "id", None),
            "name": dialog.name,
            "kind": _entity_kind(entity),
            "username": getattr(entity, "username", None),
            "unread_count": getattr(dialog, "unread_count", None),
            "pinned": bool(getattr(dialog, "pinned", False)),
        })
    return out


@mcp.tool()
async def get_recent_messages(chat: str, limit: int = 30) -> list[dict[str, Any]]:
    """Read recent messages from a chat by username, id, exact title, or title substring."""
    client = await _get_client()
    entity = await _resolve_chat(chat)
    messages = []
    async for msg in client.iter_messages(entity, limit=max(1, min(limit, 200))):
        messages.append(_message_to_dict(msg))
    return list(reversed(messages))


@mcp.tool()
async def search_messages(chat: str, query: str, limit: int = 30) -> list[dict[str, Any]]:
    """Search messages in one chat. Read-only."""
    if not query.strip():
        raise ValueError("query is required")
    client = await _get_client()
    entity = await _resolve_chat(chat)
    messages = []
    async for msg in client.iter_messages(entity, search=query, limit=max(1, min(limit, 200))):
        messages.append(_message_to_dict(msg))
    return messages


@mcp.tool()
async def get_chat_info(chat: str) -> dict[str, Any]:
    """Get metadata for one Telegram chat/user/channel."""
    entity = await _resolve_chat(chat)
    return {
        "id": get_peer_id(entity),
        "raw_id": getattr(entity, "id", None),
        "name": _entity_name(entity),
        "kind": _entity_kind(entity),
        "username": getattr(entity, "username", None),
        "verified": bool(getattr(entity, "verified", False)),
        "scam": bool(getattr(entity, "scam", False)),
        "fake": bool(getattr(entity, "fake", False)),
    }


if __name__ == "__main__":
    mcp.run()
