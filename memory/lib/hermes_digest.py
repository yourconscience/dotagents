#!/usr/bin/env python3
import json
import os
import re
import subprocess
import sys
from datetime import datetime
from pathlib import Path

SESSION_MARKER_RE = re.compile(
    r"<!-- hermes-session:(?P<sid>[^:]+):start -->.*?<!-- hermes-session:(?P=sid):end -->\n?",
    re.DOTALL,
)
PATH_RE = re.compile(r"(?:~?/[^\s`'\"\\]+|/Users/[^\s`'\"\\]+|\./[^\s`'\"\\]+)")


def truncate(text: str, limit: int = 220) -> str:
    text = re.sub(r"\s+", " ", text).strip()
    if len(text) <= limit:
        return text
    return text[: limit - 1].rstrip() + "…"


def nonempty_assistant_text(messages):
    for msg in reversed(messages):
        if msg.get("role") != "assistant":
            continue
        content = msg.get("content")
        if isinstance(content, str) and content.strip():
            return content.strip()
    return ""


def collect_user_turns(messages):
    turns = []
    for msg in messages:
        if msg.get("role") != "user":
            continue
        content = msg.get("content")
        if not isinstance(content, str):
            continue
        text = content.strip()
        if not text:
            continue
        turns.append(text)
    return turns


def extract_paths(messages):
    found = []
    seen = set()
    for msg in messages:
        content = msg.get("content")
        if not isinstance(content, str):
            continue
        for match in PATH_RE.findall(content):
            if match.startswith('/Users/') or match.startswith('~/') or match.startswith('./'):
                if match not in seen:
                    seen.add(match)
                    found.append(match)
        if len(found) >= 8:
            break
    return found[:8]


def build_block(data):
    session_id = data["session_id"]
    started = data.get("session_start") or datetime.utcnow().isoformat()
    started_dt = datetime.fromisoformat(started)
    started_human = started_dt.strftime("%Y-%m-%d %H:%M")
    messages = data.get("messages") or []
    users = collect_user_turns(messages)
    final_assistant = nonempty_assistant_text(messages)
    paths = extract_paths(messages)
    model = data.get("model") or ""
    platform = data.get("platform") or ""

    lines = [
        f"<!-- hermes-session:{session_id}:start -->",
        f"## Hermes session {started_human} - {session_id}",
        "",
    ]
    meta = []
    if platform:
        meta.append(f"platform: {platform}")
    if model:
        meta.append(f"model: {model}")
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
    lines.extend([
        f"- source transcript: `~/.hermes/sessions/session_{session_id}.json`",
        "",
        f"<!-- hermes-session:{session_id}:end -->",
        "",
    ])
    return "\n".join(lines)


def update_daily_file(ai_dir: Path, data):
    started = data.get("session_start") or datetime.utcnow().isoformat()
    day = datetime.fromisoformat(started).strftime("%Y-%m-%d")
    target = ai_dir / f"{day}.md"
    existing = target.read_text(encoding="utf-8") if target.exists() else ""
    existing = SESSION_MARKER_RE.sub("", existing).rstrip()
    block = build_block(data).rstrip() + "\n"
    if existing:
        new_content = existing + "\n\n" + block
    else:
        new_content = block
    target.write_text(new_content, encoding="utf-8")
    return target


def ai_index_paths(ai_dir: Path):
    paths = []
    for pattern in ("*.md", "*.markdown"):
        paths.extend(str(p) for p in sorted(ai_dir.glob(pattern)) if p.is_file())
    return paths


def reindex(notes_dir: str, profile_dir: str, ai_dir: Path, collection: str):
    subprocess.run(
        [
            "memsearch",
            "index",
            notes_dir,
            profile_dir,
            *ai_index_paths(ai_dir),
            "--collection",
            collection,
        ],
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
        check=False,
    )


def main():
    payload = json.load(sys.stdin)
    session_id = payload.get("session_id")
    if not session_id:
        print(json.dumps({"action": "continue"}))
        return

    hermes_home = Path(os.path.expanduser(os.environ.get("HERMES_HOME", "~/.hermes")))
    session_path = hermes_home / "sessions" / f"session_{session_id}.json"
    if not session_path.exists():
        print(json.dumps({"action": "continue", "message": f"session log not found: {session_path}"}))
        return

    with session_path.open(encoding="utf-8") as f:
        data = json.load(f)

    knowledge_dir = Path(os.path.expanduser(os.environ.get("KNOWLEDGE_DIR", "~/Workspace/knowledge")))
    ai_dir = Path(os.path.expanduser(os.environ.get("SESSIONS_DIR", str(knowledge_dir / "sessions"))))
    ai_dir.mkdir(parents=True, exist_ok=True)
    target = update_daily_file(ai_dir, data)

    notes_dir = os.path.expanduser(os.environ.get("MEMSEARCH_NOTES_DIR", "~/Workspace/knowledge/notes"))
    profile_dir = os.path.expanduser(os.environ.get("MEMSEARCH_PROFILE_DIR", "~/Workspace/knowledge/profile"))
    collection = os.environ.get("MEMSEARCH_COLLECTION", "ai")
    reindex(notes_dir, profile_dir, ai_dir, collection)

    print(json.dumps({
        "action": "continue",
        "message": f"memsearch updated from Hermes session {session_id}",
        "output_file": str(target),
    }))


if __name__ == "__main__":
    main()
