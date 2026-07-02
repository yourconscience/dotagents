#!/usr/bin/env python3
"""Read-only Telegram MCP server with singleton daemon.

A single daemon process holds the Telethon MTProto session and listens on a
Unix domain socket. MCP server instances (one per Claude Code session) proxy
tool calls through the socket. This prevents auth key conflicts from
concurrent session file access.

  server.py            MCP stdio server (default), proxies to daemon
  server.py --daemon   background process, owns Telethon connection
  server.py --stop     kill a running daemon
"""
from __future__ import annotations

import asyncio
import fcntl
import json
import os
import re
import signal
import subprocess
import sys
import time
from datetime import timezone
from pathlib import Path
from typing import Any

from dotenv import load_dotenv
from mcp.server.fastmcp import FastMCP

ROOT = Path(__file__).resolve().parent
ENV_PATH = Path(os.getenv("TELEGRAM_READONLY_ENV", str(ROOT / ".env"))).expanduser()


def _load_env() -> None:
    if ENV_PATH.exists():
        load_dotenv(ENV_PATH, override=False)


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


_STATE_DIR = Path(os.getenv(
    "TELEGRAM_SESSION_PATH",
    str(Path.home() / ".local" / "share" / "dotagents" / "telegram-readonly" / "telegram.session"),
)).expanduser().parent
_SOCK_PATH = _STATE_DIR / "daemon.sock"
_LOCK_PATH = _STATE_DIR / "daemon.lock"
_PID_PATH = _STATE_DIR / "daemon.pid"
_LOG_PATH = _STATE_DIR / "daemon.log"
_IDLE_TIMEOUT = 1800
_MAX_MESSAGES_PER_CALL = 100


# ── Helpers (shared by daemon) ───────────────────────────────────────────

def _entity_name(entity: Any) -> str:
    from telethon.tl.types import User
    if isinstance(entity, User):
        parts = [entity.first_name or "", entity.last_name or ""]
        name = " ".join(p for p in parts if p).strip()
        return name or entity.username or str(entity.id)
    return getattr(entity, "title", None) or getattr(entity, "username", None) or str(getattr(entity, "id", "unknown"))


def _entity_kind(entity: Any) -> str:
    from telethon.tl.types import User, Chat, Channel
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


async def _resolve_chat(client: Any, chat: str) -> Any:
    chat = str(chat).strip()
    if not chat:
        raise ValueError("chat is required: username, numeric id, exact title, or substring")
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
    substring_match = None
    async for dialog in client.iter_dialogs():
        if not dialog.name:
            continue
        if dialog.name.lower() == low:
            return dialog.entity
        if substring_match is None and low in dialog.name.lower():
            substring_match = dialog.entity
    if substring_match is not None:
        return substring_match
    raise ValueError(f"Could not resolve Telegram chat: {chat!r}")


# ── Daemon mode ──────────────────────────────────────────────────────────

async def _daemon_main() -> None:
    from telethon import TelegramClient

    _STATE_DIR.mkdir(parents=True, exist_ok=True)

    lock_fd = open(_LOCK_PATH, "w")
    try:
        fcntl.flock(lock_fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
    except BlockingIOError:
        print("Another daemon is already running", file=sys.stderr)
        sys.exit(1)

    if _SOCK_PATH.exists():
        _SOCK_PATH.unlink()

    _PID_PATH.write_text(str(os.getpid()))

    api_id, api_hash, session_path = _require_env()
    Path(session_path).parent.mkdir(parents=True, exist_ok=True)
    client = TelegramClient(session_path, api_id, api_hash)
    await client.connect()
    if not await client.is_user_authorized():
        await client.disconnect()
        raise RuntimeError(f"Not authorized. Run: cd {ROOT} && uv run python login.py")

    last_activity = time.monotonic()
    active_conns = 0

    async def handle_method(method: str, params: dict) -> Any:
        nonlocal last_activity
        last_activity = time.monotonic()

        if method == "ping":
            return "pong"
        if method == "list_dialogs":
            out = []
            q = (params.get("query") or "").lower() or None
            async for dialog in client.iter_dialogs(limit=max(1, min(params.get("limit", 50), 200))):
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
        if method == "get_recent_messages":
            entity = await _resolve_chat(client, params["chat"])
            msgs = []
            kwargs = {
                "limit": max(1, min(params.get("limit", 30), _MAX_MESSAGES_PER_CALL)),
            }
            if params.get("offset_id") is not None:
                kwargs["offset_id"] = int(params["offset_id"])
            async for msg in client.iter_messages(entity, **kwargs):
                msgs.append(_message_to_dict(msg))
            return list(reversed(msgs))
        if method == "search_messages":
            q = params.get("query", "").strip()
            if not q:
                raise ValueError("query is required")
            entity = await _resolve_chat(client, params["chat"])
            msgs = []
            async for msg in client.iter_messages(entity, search=q, limit=max(1, min(params.get("limit", 30), 200))):
                msgs.append(_message_to_dict(msg))
            return msgs
        if method == "get_chat_info":
            from telethon.utils import get_peer_id
            entity = await _resolve_chat(client, params["chat"])
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
        raise ValueError(f"Unknown method: {method}")

    async def handle_client(reader: asyncio.StreamReader, writer: asyncio.StreamWriter):
        nonlocal active_conns
        active_conns += 1
        try:
            while True:
                line = await reader.readline()
                if not line:
                    break
                req_id = None
                try:
                    req = json.loads(line)
                    req_id = req.get("id")
                    result = await handle_method(req["method"], req.get("params", {}))
                    resp = {"id": req_id, "result": result}
                except Exception as e:
                    resp = {"id": req_id, "error": f"{type(e).__name__}: {e}"}
                writer.write(json.dumps(resp).encode() + b"\n")
                await writer.drain()
        except (ConnectionResetError, BrokenPipeError):
            pass
        finally:
            active_conns -= 1
            writer.close()

    server = await asyncio.start_unix_server(handle_client, path=str(_SOCK_PATH))
    os.chmod(str(_SOCK_PATH), 0o600)
    print(f"telegram-readonly daemon started pid={os.getpid()}", file=sys.stderr)

    shutdown = asyncio.Event()

    def on_signal():
        shutdown.set()

    loop = asyncio.get_running_loop()
    for sig in (signal.SIGTERM, signal.SIGINT):
        loop.add_signal_handler(sig, on_signal)

    async def watchdog():
        while not shutdown.is_set():
            await asyncio.sleep(60)
            if time.monotonic() - last_activity > _IDLE_TIMEOUT and active_conns == 0:
                print("Idle timeout, shutting down", file=sys.stderr)
                shutdown.set()
                return

    watchdog_task = asyncio.create_task(watchdog())
    serve_task = asyncio.create_task(server.serve_forever())

    await shutdown.wait()
    serve_task.cancel()
    watchdog_task.cancel()
    server.close()
    await client.disconnect()
    _SOCK_PATH.unlink(missing_ok=True)
    _PID_PATH.unlink(missing_ok=True)
    print("daemon stopped", file=sys.stderr)


def _stop_daemon() -> None:
    if not _PID_PATH.exists():
        print("No daemon PID file found")
        return
    pid = int(_PID_PATH.read_text().strip())
    try:
        os.kill(pid, signal.SIGTERM)
        print(f"Sent SIGTERM to daemon pid={pid}")
    except ProcessLookupError:
        print(f"Daemon pid={pid} not running, cleaning up")
        _PID_PATH.unlink(missing_ok=True)
        _SOCK_PATH.unlink(missing_ok=True)


# ── MCP proxy mode ──────────────────────────────────────────────────────

_req_id = 0


async def _ensure_daemon() -> None:
    try:
        r, w = await asyncio.open_unix_connection(str(_SOCK_PATH))
        w.close()
        await w.wait_closed()
        return
    except (FileNotFoundError, ConnectionRefusedError, OSError):
        pass

    _STATE_DIR.mkdir(parents=True, exist_ok=True)
    log_fd = open(_LOG_PATH, "a")
    subprocess.Popen(
        [sys.executable, str(ROOT / "server.py"), "--daemon"],
        stdin=subprocess.DEVNULL,
        stdout=log_fd,
        stderr=log_fd,
        start_new_session=True,
    )
    log_fd.close()

    for _ in range(50):
        await asyncio.sleep(0.2)
        try:
            r, w = await asyncio.open_unix_connection(str(_SOCK_PATH))
            w.close()
            await w.wait_closed()
            return
        except (FileNotFoundError, ConnectionRefusedError, OSError):
            continue
    raise RuntimeError(
        f"Failed to start daemon. Check {_LOG_PATH}"
    )


async def _proxy_call(method: str, params: dict) -> Any:
    global _req_id
    await _ensure_daemon()
    r, w = await asyncio.open_unix_connection(str(_SOCK_PATH))
    _req_id += 1
    req = {"id": _req_id, "method": method, "params": params}
    w.write(json.dumps(req).encode() + b"\n")
    await w.drain()
    line = await r.readline()
    w.close()
    await w.wait_closed()
    if not line:
        raise RuntimeError("Daemon closed connection unexpectedly")
    resp = json.loads(line)
    if "error" in resp:
        raise RuntimeError(resp["error"])
    return resp["result"]


mcp = FastMCP("telegram-readonly")


@mcp.tool()
async def list_dialogs(limit: int = 50, query: str | None = None) -> Any:
    """List recent Telegram dialogs. Optional case-insensitive title/name filter."""
    params: dict = {"limit": limit}
    if query is not None:
        params["query"] = query
    return await _proxy_call("list_dialogs", params)


@mcp.tool()
async def get_recent_messages(chat: str, limit: int = 30, offset_id: int | None = None) -> Any:
    """Read recent messages from a chat by username, id, exact title, or title substring."""
    params: dict = {"chat": chat, "limit": limit}
    if offset_id is not None:
        params["offset_id"] = offset_id
    return await _proxy_call("get_recent_messages", params)


@mcp.tool()
async def search_messages(chat: str, query: str, limit: int = 30) -> Any:
    """Search messages in one chat. Read-only."""
    return await _proxy_call("search_messages", {"chat": chat, "query": query, "limit": limit})


@mcp.tool()
async def get_chat_info(chat: str) -> Any:
    """Get metadata for one Telegram chat/user/channel."""
    return await _proxy_call("get_chat_info", {"chat": chat})


if __name__ == "__main__":
    if "--daemon" in sys.argv:
        asyncio.run(_daemon_main())
    elif "--stop" in sys.argv:
        _stop_daemon()
    else:
        mcp.run()
