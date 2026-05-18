#!/usr/bin/env python3
"""One-time Telegram login for the read-only MCP server.

Run this yourself from a terminal after creating `.env` from `.env.example` and
filling TELEGRAM_API_ID / TELEGRAM_API_HASH from https://my.telegram.org.
The script may ask for your phone, login code, and 2FA password. Do not paste
those secrets into an agent chat.
"""
from __future__ import annotations

import asyncio
import os
from pathlib import Path

from telethon import TelegramClient

ROOT = Path(__file__).resolve().parent
ENV_PATH = Path(os.getenv("TELEGRAM_READONLY_ENV", str(ROOT / ".env"))).expanduser()


def load_env() -> None:
    if not ENV_PATH.exists():
        raise SystemExit(f"Missing {ENV_PATH}. Copy .env.example to .env and fill it.")
    for line in ENV_PATH.read_text().splitlines():
        line = line.strip()
        if not line or line.startswith("#") or "=" not in line:
            continue
        key, value = line.split("=", 1)
        os.environ.setdefault(key.strip(), value.strip())


async def main() -> None:
    load_env()
    api_id = os.getenv("TELEGRAM_API_ID")
    api_hash = os.getenv("TELEGRAM_API_HASH")
    session_path = os.getenv(
        "TELEGRAM_SESSION_PATH",
        str(Path.home() / ".local" / "share" / "dotagents" / "telegram-readonly" / "telegram.session"),
    )
    if not api_id or not api_hash:
        raise SystemExit("TELEGRAM_API_ID and TELEGRAM_API_HASH are required in .env")
    Path(session_path).expanduser().parent.mkdir(parents=True, exist_ok=True)
    client = TelegramClient(str(Path(session_path).expanduser()), int(api_id), api_hash)
    await client.start()
    me = await client.get_me()
    print(f"Logged in as {getattr(me, 'username', None) or getattr(me, 'first_name', '')} ({me.id})")
    await client.disconnect()


if __name__ == "__main__":
    asyncio.run(main())
