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
import re
import shlex
from pathlib import Path

from telethon import TelegramClient

ROOT = Path(__file__).resolve().parent
ENV_PATH = Path(os.getenv("TELEGRAM_READONLY_ENV", str(ROOT / ".env"))).expanduser()
ENV_KEY_RE = re.compile(r"^[A-Za-z_][A-Za-z0-9_]*$")


def strip_unquoted_comment(value: str) -> str:
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


def parse_env_line(line: str) -> tuple[str, str] | None:
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
    parts = shlex.split(strip_unquoted_comment(value), comments=False, posix=True)
    return key, " ".join(parts)


def load_env() -> None:
    if not ENV_PATH.exists():
        raise SystemExit(f"Missing {ENV_PATH}. Copy .env.example to .env and fill it.")
    for line_no, line in enumerate(ENV_PATH.read_text().splitlines(), 1):
        try:
            parsed = parse_env_line(line)
        except ValueError as err:
            raise SystemExit(f"Invalid .env line {line_no} in {ENV_PATH}: {err}") from err
        if parsed is None:
            continue
        key, value = parsed
        os.environ.setdefault(key, value)


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
