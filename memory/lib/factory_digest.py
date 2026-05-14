#!/usr/bin/env python3
import json
import os
import re
import subprocess
import sys
from datetime import datetime, timezone
from pathlib import Path
from typing import Optional

PATH_RE = re.compile(r"(?:~?/[^\s`'\"\\]+|/Users/[^\s`'\"\\]+|\./[^\s`'\"\\]+)")
SECRET_RE = re.compile(
    r"(?i)\b((?:api[_-]?key|secret|token|password|authorization)\s*[:=]\s*)([^\s`'\"\\]{8,})"
)


def truncate(text: str, limit: int = 220) -> str:
    text = SECRET_RE.sub(r"\1[REDACTED]", text)
    text = re.sub(r"\s+", " ", text).strip()
    if len(text) <= limit:
        return text
    return text[: limit - 1].rstrip() + "…"


def parse_timestamp(value: Optional[str]) -> datetime:
    if value:
        try:
            return datetime.fromisoformat(value.replace("Z", "+00:00"))
        except ValueError:
            pass
    return datetime.now(timezone.utc)


def content_to_text(content) -> str:
    if isinstance(content, str):
        return content
    if isinstance(content, list):
        parts = []
        for item in content:
            if isinstance(item, dict):
                text = item.get("text") or item.get("content")
                if isinstance(text, str):
                    parts.append(text)
            elif isinstance(item, str):
                parts.append(item)
        return "\n".join(parts)
    return ""


def read_transcript(path: Path):
    session = {}
    messages = []
    session_end = None
    with path.open(encoding="utf-8") as f:
        for line in f:
            if not line.strip():
                continue
            try:
                event = json.loads(line)
            except json.JSONDecodeError:
                continue
            event_type = event.get("type")
            if event_type == "session_start":
                session = event
            elif event_type == "session_end":
                session_end = event
            elif event_type == "message":
                message = event.get("message")
                if isinstance(message, dict):
                    if message.get("visibility") == "llm_only":
                        continue
                    messages.append(
                        {
                            "role": message.get("role"),
                            "content": content_to_text(message.get("content")),
                            "timestamp": event.get("timestamp"),
                        }
                    )
    return session, messages, session_end


def collect_user_turns(messages):
    return [
        msg["content"].strip()
        for msg in messages
        if msg.get("role") == "user" and isinstance(msg.get("content"), str) and msg["content"].strip()
    ]


def nonempty_assistant_text(messages):
    for msg in reversed(messages):
        if msg.get("role") == "assistant" and isinstance(msg.get("content"), str) and msg["content"].strip():
            return msg["content"].strip()
    return ""


def extract_paths(messages):
    found = []
    seen = set()
    for msg in messages:
        content = msg.get("content")
        if not isinstance(content, str):
            continue
        for match in PATH_RE.findall(content):
            cleaned = match.rstrip(".,;:)")
            if cleaned.startswith(("/Users/", "~/", "./")) and cleaned not in seen:
                seen.add(cleaned)
                found.append(cleaned)
        if len(found) >= 8:
            break
    return found[:8]


def build_block(payload, transcript_path: Path, session, messages):
    session_id = payload.get("session_id") or session.get("id") or transcript_path.stem
    started = parse_timestamp(session.get("timestamp") or (messages[0].get("timestamp") if messages else None))
    started_human = started.strftime("%Y-%m-%d %H:%M")
    users = collect_user_turns(messages)
    final_assistant = nonempty_assistant_text(messages)
    paths = extract_paths(messages)
    cwd = payload.get("cwd") or session.get("cwd") or ""
    title = session.get("sessionTitle") or session.get("title") or ""

    lines = [
        f"<!-- droid-session:{session_id}:start -->",
        f"## Droid session {started_human} - {session_id}",
        "",
    ]
    meta = []
    if title:
        meta.append(f"title: {truncate(title, 180)}")
    if cwd:
        meta.append(f"cwd: `{cwd}`")
    if meta:
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
        for p in paths:
            lines.append(f"  - `{p}`")
    lines.extend(
        [
            f"- source transcript: `{transcript_path}`",
            "",
            f"<!-- droid-session:{session_id}:end -->",
            "",
        ]
    )
    return "\n".join(lines), started


def update_daily_file(ai_dir: Path, payload, transcript_path: Path, session, messages):
    block, started = build_block(payload, transcript_path, session, messages)
    session_id = payload.get("session_id") or session.get("id") or transcript_path.stem
    target = ai_dir / f"{started.strftime('%Y-%m-%d')}.md"
    existing = target.read_text(encoding="utf-8") if target.exists() else ""
    marker_re = re.compile(
        rf"<!-- droid-session:{re.escape(session_id)}:start -->.*?<!-- droid-session:{re.escape(session_id)}:end -->\n?",
        re.DOTALL,
    )
    existing = marker_re.sub("", existing).rstrip()
    block = block.rstrip() + "\n"
    target.write_text((existing + "\n\n" + block if existing else block), encoding="utf-8")
    return target


def ai_index_paths(ai_dir: Path):
    paths = []
    for pattern in ("*.md", "*.markdown"):
        paths.extend(str(p) for p in sorted(ai_dir.glob(pattern)) if p.is_file())
    return paths


def reindex(notes_dir: str, profile_dir: str, ai_dir: Path, collection: str):
    paths = [notes_dir, profile_dir, *ai_index_paths(ai_dir)]
    result = subprocess.run(
        ["memsearch", "index", *paths, "--collection", collection],
        capture_output=True,
        text=True,
        check=False,
    )
    if result.returncode != 0:
        print(f"memsearch reindex warning: {result.stderr.strip()}", file=sys.stderr)


def main():
    try:
        payload = json.load(sys.stdin)
    except json.JSONDecodeError:
        print(json.dumps({"continue": True, "suppressOutput": True}))
        return

    transcript = payload.get("transcript_path")
    if not transcript:
        print(json.dumps({"continue": True, "suppressOutput": True}))
        return

    transcript_path = Path(os.path.expanduser(transcript))
    if not transcript_path.exists():
        print(json.dumps({"continue": True, "suppressOutput": True}))
        return

    session, messages, _ = read_transcript(transcript_path)
    if not messages:
        print(json.dumps({"continue": True, "suppressOutput": True}))
        return

    knowledge_dir = Path(os.path.expanduser(os.environ.get("KNOWLEDGE_DIR", "~/Workspace/knowledge")))
    ai_dir = Path(os.path.expanduser(os.environ.get("SESSIONS_DIR", str(knowledge_dir / "sessions"))))
    ai_dir.mkdir(parents=True, exist_ok=True)
    target = update_daily_file(ai_dir, payload, transcript_path, session, messages)

    notes_dir = os.path.expanduser(os.environ.get("MEMSEARCH_NOTES_DIR", "~/Workspace/knowledge/notes"))
    profile_dir = os.path.expanduser(os.environ.get("MEMSEARCH_PROFILE_DIR", "~/Workspace/knowledge/profile"))
    collection = os.environ.get("MEMSEARCH_COLLECTION", "ai")
    reindex(notes_dir, profile_dir, ai_dir, collection)

    print(
        json.dumps(
            {
                "continue": True,
                "suppressOutput": True,
                "systemMessage": f"memsearch updated from Droid session {payload.get('session_id') or session.get('id')}",
                "output_file": str(target),
            }
        )
    )


if __name__ == "__main__":
    main()
