#!/usr/bin/env python3
"""Append an Amp session digest to the shared knowledge vault."""

from __future__ import annotations

import json
import os
import re
import subprocess
import sys
from datetime import datetime
from datetime import timezone
from pathlib import Path
from typing import Any

from safety import atomic_write_text, locked_directory, redact_text, safe_identifier, truncate_redacted


PATH_RE = re.compile(r"(?:~?/[^\s`'\"\\]+|/Users/[^\s`'\"\\]+|\./[^\s`'\"\\]+)")


def truncate(text: str, limit: int = 220) -> str:
    return truncate_redacted(text, limit)


def parse_start(value: Any) -> datetime:
    if isinstance(value, str) and value.strip():
        normalized = value.strip().replace("Z", "+00:00")
        try:
            return datetime.fromisoformat(normalized)
        except ValueError:
            pass
    return datetime.now(timezone.utc)


def content_text(content: Any) -> str:
    if isinstance(content, str):
        return content
    if isinstance(content, list):
        parts: list[str] = []
        for item in content:
            if isinstance(item, dict):
                text = item.get("text") or item.get("content")
                if isinstance(text, str):
                    parts.append(text)
        return "\n".join(parts)
    return ""


def collect_user_turns(messages: list[dict[str, Any]]) -> list[str]:
    turns: list[str] = []
    for msg in messages:
        if msg.get("role") != "user":
            continue
        text = content_text(msg.get("content")).strip()
        if text:
            turns.append(text)
    return turns


def nonempty_assistant_text(messages: list[dict[str, Any]]) -> str:
    for msg in reversed(messages):
        if msg.get("role") != "assistant":
            continue
        text = content_text(msg.get("content")).strip()
        if text:
            return text
    return ""


def extract_paths(messages: list[dict[str, Any]]) -> list[str]:
    found: list[str] = []
    seen: set[str] = set()
    for msg in messages:
        text = content_text(msg.get("content"))
        for match in PATH_RE.findall(text):
            cleaned = redact_text(match.rstrip(".,;:)"))
            if cleaned.startswith("/Users/") or cleaned.startswith("~/") or cleaned.startswith("./"):
                if cleaned not in seen:
                    seen.add(cleaned)
                    found.append(cleaned)
        if len(found) >= 8:
            break
    return found[:8]


def build_block(data: dict[str, Any]) -> str:
    session_id = safe_identifier(data.get("session_id") or data.get("amp_thread_id") or "unknown")
    started = parse_start(data.get("session_start"))
    started_human = started.strftime("%Y-%m-%d %H:%M")
    messages = data.get("messages") if isinstance(data.get("messages"), list) else []
    users = collect_user_turns(messages)
    final_assistant = nonempty_assistant_text(messages)
    paths = extract_paths(messages)
    model = truncate(str(data.get("model") or ""), 120)

    lines = [
        f"<!-- amp-session:{session_id}:start -->",
        f"## Amp session {started_human} - {session_id}",
        "",
    ]
    meta = ["platform: amp"]
    if model:
        meta.append(f"model: {model}")
    lines.append(f"- {'; '.join(meta)}")
    if users:
        lines.append(f"- first request: {truncate(users[0], 260)}")
    if len(users) > 1:
        lines.append("- user follow-ups:")
        for text in users[1:6]:
            lines.append(f"  - {truncate(text, 220)}")
    if final_assistant:
        lines.append(f"- final assistant output: {truncate(final_assistant, 420)}")
    if paths:
        lines.append("- paths mentioned:")
        for path in paths:
            lines.append(f"  - `{truncate(path, 180)}`")
    lines.extend(
        [
            "",
            f"<!-- amp-session:{session_id}:end -->",
            "",
        ]
    )
    return "\n".join(lines)


def update_daily_file(sessions_dir: Path, data: dict[str, Any]) -> Path:
    started = parse_start(data.get("session_start"))
    target = sessions_dir / f"{started.strftime('%Y-%m-%d')}.md"
    session_id = safe_identifier(data.get("session_id") or data.get("amp_thread_id") or "unknown")
    session_marker_re = re.compile(
        rf"<!-- amp-session:{re.escape(session_id)}:start -->.*?<!-- amp-session:{re.escape(session_id)}:end -->\n?",
        re.DOTALL,
    )

    with locked_directory(sessions_dir):
        existing = ""
        session_files = sorted({*sessions_dir.glob("*.md"), *sessions_dir.glob("*.markdown")})
        for path in session_files:
            text = path.read_text(encoding="utf-8")
            cleaned = session_marker_re.sub("", text).rstrip()
            if path == target:
                existing = cleaned
            elif cleaned != text.rstrip():
                atomic_write_text(path, cleaned + "\n" if cleaned else "")
        if not existing and target.exists() and target not in session_files:
            existing = session_marker_re.sub("", target.read_text(encoding="utf-8")).rstrip()

        block = build_block(data).rstrip() + "\n"
        atomic_write_text(target, (existing + "\n\n" + block).lstrip() if existing else block)
    return target


def reindex(notes_dir: str, profile_dir: str, sessions_dir: Path, collection: str) -> None:
    paths: list[str] = []
    for pattern in ("*.md", "*.markdown"):
        paths.extend(str(path) for path in sorted(sessions_dir.glob(pattern)) if path.is_file())
    try:
        subprocess.run(
            ["memsearch", "index", notes_dir, profile_dir, *paths, "--collection", collection],
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
            check=False,
        )
    except OSError:
        pass


def main() -> None:
    payload = json.load(sys.stdin)
    session_id = payload.get("session_id") or payload.get("amp_thread_id")
    if not session_id:
        print(json.dumps({"action": "continue"}))
        return

    knowledge_dir = Path(os.path.expanduser(os.environ.get("KNOWLEDGE_DIR", "~/Workspace/knowledge")))
    sessions_dir = Path(os.path.expanduser(os.environ.get("SESSIONS_DIR", str(knowledge_dir / "sessions"))))
    notes_dir = os.path.expanduser(os.environ.get("NOTES_DIR", str(knowledge_dir / "notes")))
    profile_dir = os.path.expanduser(os.environ.get("PROFILE_DIR", str(knowledge_dir / "profile")))
    collection = os.environ.get("MEMSEARCH_COLLECTION", "ai")

    sessions_dir.mkdir(parents=True, exist_ok=True)
    target = update_daily_file(sessions_dir, payload)
    reindex(notes_dir, profile_dir, sessions_dir, collection)

    print(
        json.dumps(
            {
                "action": "continue",
                "message": f"memsearch updated from Amp session {safe_identifier(session_id)}",
                "output_file": str(target),
            }
        )
    )


if __name__ == "__main__":
    main()
