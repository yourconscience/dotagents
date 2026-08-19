#!/usr/bin/env python3
"""Basic markdown-backed memory hooks for dotagents.

This module intentionally has no memsearch or third-party dependency. It reads
hook payloads, stores compact digests under $KNOWLEDGE_DIR/sessions, and emits
Claude-compatible hook JSON on stdout.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import re
import sys
import tempfile
import unicodedata
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, TextIO

from safety import knowledge_dir_from_environment, locked_directory, redact_text, safe_identifier, truncate_redacted

MAX_DIGEST_CHARS = 3200
MAX_CONTEXT_CHARS = 6000
MAX_CONTEXT_FILES = 7
MAX_CONTEXT_FILE_CHARS = 2400

PATH_RE = re.compile(r"(?:~?/[^\s`'\"\\]+|/Users/[^\s`'\"\\]+|\./[^\s`'\"\\]+)")


class HookError(Exception):
    """Actionable hook failure."""


def _error(message: str) -> int:
    print(f"basic memory hook error: {message}", file=sys.stderr)
    return 1


def parse_payload(stdin: TextIO) -> dict[str, Any]:
    try:
        payload = json.load(stdin)
    except json.JSONDecodeError as exc:
        raise HookError(f"invalid hook JSON on stdin: {exc.msg} at line {exc.lineno} column {exc.colno}") from exc
    if not isinstance(payload, dict):
        raise HookError("hook payload must be a JSON object")
    return payload


def knowledge_dir_from_env() -> Path:
    return knowledge_dir_from_environment()


def ensure_sessions_dir(knowledge_dir: Path) -> Path:
    sessions_dir = knowledge_dir / "sessions"
    try:
        sessions_dir.mkdir(parents=True, exist_ok=True)
    except OSError as exc:
        raise HookError(f"cannot create sessions directory {sessions_dir}: {exc}") from exc
    if not sessions_dir.is_dir():
        raise HookError(f"sessions path is not a directory: {sessions_dir}")
    test_path: Path | None = None
    try:
        with tempfile.NamedTemporaryFile(
            mode="w",
            encoding="utf-8",
            dir=sessions_dir,
            prefix=".dotagents-basic-memory-write-test-",
            delete=False,
        ) as handle:
            test_path = Path(handle.name)
        test_path.unlink()
    except OSError as exc:
        if test_path is not None:
            try:
                test_path.unlink(missing_ok=True)
            except OSError:
                pass
        raise HookError(f"sessions directory is not writable: {sessions_dir}: {exc}") from exc
    return sessions_dir


def require_readable_sessions_dir(knowledge_dir: Path) -> Path | None:
    sessions_dir = knowledge_dir / "sessions"
    if not sessions_dir.exists():
        return None
    if not sessions_dir.is_dir():
        raise HookError(f"sessions path is not a directory: {sessions_dir}")
    try:
        next(sessions_dir.iterdir(), None)
    except OSError as exc:
        raise HookError(f"sessions directory is not readable: {sessions_dir}: {exc}") from exc
    return sessions_dir


def content_text(content: Any) -> str:
    if isinstance(content, str):
        return content
    if isinstance(content, list):
        parts: list[str] = []
        for item in content:
            if isinstance(item, str):
                parts.append(item)
            elif isinstance(item, dict):
                text = item.get("text") or item.get("content")
                if isinstance(text, str):
                    parts.append(text)
        return "\n".join(parts)
    if isinstance(content, dict):
        text = content.get("text") or content.get("content")
        if isinstance(text, str):
            return text
    return ""


def redacted(text: str) -> str:
    return redact_text(text)


def truncate(text: str, limit: int) -> str:
    return truncate_redacted(text, limit)


def parse_timestamp(value: Any) -> datetime | None:
    if isinstance(value, (int, float)):
        try:
            return datetime.fromtimestamp(value, tz=timezone.utc)
        except (OSError, OverflowError, ValueError):
            return None
    if not isinstance(value, str) or not value.strip():
        return None
    normalized = value.strip().replace("Z", "+00:00")
    try:
        parsed = datetime.fromisoformat(normalized)
    except ValueError:
        return None
    if parsed.tzinfo is None:
        return parsed.replace(tzinfo=timezone.utc)
    return parsed.astimezone(timezone.utc)


def first_timestamp(*items: Any) -> datetime:
    for item in items:
        parsed = parse_timestamp(item)
        if parsed is not None:
            return parsed
    return datetime.now(timezone.utc)


def message_from_record(record: dict[str, Any]) -> dict[str, Any] | None:
    role = record.get("role")
    content = record.get("content")
    timestamp = record.get("timestamp") or record.get("created_at")

    nested = record.get("message")
    if isinstance(nested, dict):
        role = nested.get("role") or role
        content = nested.get("content") if "content" in nested else content
        timestamp = nested.get("timestamp") or timestamp

    if role is None and record.get("type") in {"user", "assistant", "system"}:
        role = record.get("type")
    if role is None or content is None:
        return None
    text = content_text(content).strip()
    if not text:
        return None
    return {"role": str(role), "content": text, "timestamp": timestamp}


def inline_messages(payload: dict[str, Any]) -> list[dict[str, Any]]:
    messages = payload.get("messages")
    if not isinstance(messages, list):
        return []
    parsed: list[dict[str, Any]] = []
    for item in messages:
        if isinstance(item, dict):
            message = message_from_record(item)
            if message is not None:
                parsed.append(message)
    return parsed


def read_transcript(path: Path) -> tuple[list[dict[str, Any]], datetime | None, str | None]:
    if not path.exists():
        raise HookError(f"transcript_path does not exist: {path}")
    if not path.is_file():
        raise HookError(f"transcript_path is not a file: {path}")

    messages: list[dict[str, Any]] = []
    started: datetime | None = None
    transcript_session_id: str | None = None
    try:
        with path.open(encoding="utf-8") as handle:
            for line_number, line in enumerate(handle, start=1):
                raw = line.strip()
                if not raw:
                    continue
                try:
                    record = json.loads(raw)
                except json.JSONDecodeError as exc:
                    raise HookError(f"invalid JSONL transcript at {path}:{line_number}: {exc.msg}") from exc
                if not isinstance(record, dict):
                    continue
                if started is None:
                    started = parse_timestamp(record.get("timestamp") or record.get("created_at"))
                if transcript_session_id is None:
                    candidate = record.get("session_id") or record.get("sessionId") or record.get("id")
                    if candidate:
                        transcript_session_id = str(candidate)
                message = message_from_record(record)
                if message is not None and message.get("role") != "system":
                    messages.append(message)
    except OSError as exc:
        raise HookError(f"cannot read transcript_path {path}: {exc}") from exc
    return messages, started, transcript_session_id


def collect_messages(payload: dict[str, Any]) -> tuple[list[dict[str, Any]], datetime | None, str | None]:
    messages = inline_messages(payload)
    transcript_started = None
    transcript_session_id = None
    transcript = payload.get("transcript_path")
    if isinstance(transcript, str) and transcript.strip():
        transcript_path = Path(os.path.expanduser(transcript)).resolve()
        transcript_messages, transcript_started, transcript_session_id = read_transcript(transcript_path)
        if transcript_messages:
            messages = transcript_messages
    return messages, transcript_started, transcript_session_id


def stable_identifier(payload: dict[str, Any], transcript_session_id: str | None) -> str | None:
    for key in ("event_id", "hook_event_id", "session_id", "sessionId", "amp_thread_id", "conversation_id", "thread_id"):
        value = payload.get(key)
        if value:
            return sanitize_identifier(str(value))
    if transcript_session_id:
        return sanitize_identifier(transcript_session_id)
    transcript = payload.get("transcript_path")
    if isinstance(transcript, str) and transcript.strip():
        return sanitize_identifier(Path(transcript).stem)
    return None


def sanitize_identifier(value: str) -> str:
    return safe_identifier(value)


def collect_user_turns(messages: list[dict[str, Any]]) -> list[str]:
    return [msg["content"] for msg in messages if msg.get("role") == "user" and isinstance(msg.get("content"), str)]


def last_assistant_text(messages: list[dict[str, Any]]) -> str:
    for msg in reversed(messages):
        if msg.get("role") == "assistant" and isinstance(msg.get("content"), str) and msg["content"].strip():
            return msg["content"]
    return ""


def extract_paths(messages: list[dict[str, Any]], payload: dict[str, Any]) -> list[str]:
    found: list[str] = []
    seen: set[str] = set()
    for candidate in (payload.get("cwd"), payload.get("workspace")):
        if isinstance(candidate, str) and candidate.strip():
            cleaned_candidate = redact_text(candidate)
            if cleaned_candidate not in seen:
                seen.add(cleaned_candidate)
                found.append(cleaned_candidate)
    for msg in messages:
        content = msg.get("content")
        if not isinstance(content, str):
            continue
        for match in PATH_RE.findall(content):
            cleaned = redact_text(match.rstrip(".,;:)"))
            if cleaned.startswith(("/Users/", "~/", "./")) and cleaned not in seen:
                seen.add(cleaned)
                found.append(cleaned)
        if len(found) >= 8:
            break
    return found[:8]


def digest_started(payload: dict[str, Any], messages: list[dict[str, Any]], transcript_started: datetime | None) -> datetime:
    message_start = None
    if messages:
        message_start = parse_timestamp(messages[0].get("timestamp"))
    return first_timestamp(
        payload.get("session_start"),
        payload.get("started_at"),
        payload.get("timestamp"),
        payload.get("created_at"),
        transcript_started.isoformat() if transcript_started else None,
        message_start.isoformat() if message_start else None,
    )


def build_digest(payload: dict[str, Any], messages: list[dict[str, Any]], started: datetime, session_id: str | None) -> str:
    display_id = session_id or "unidentified"
    started_human = started.strftime("%Y-%m-%d %H:%M UTC")
    users = collect_user_turns(messages)
    assistant = last_assistant_text(messages)
    paths = extract_paths(messages, payload)
    platform = payload.get("platform") or payload.get("agent") or payload.get("hook_event_name") or "basic"
    model = payload.get("model")

    lines = []
    if session_id:
        lines.append(f"<!-- basic-memory-session:{session_id}:start -->")
    lines.extend([f"## Session {started_human} - {display_id}", ""])

    meta = [f"source: {truncate(str(platform), 80)}"]
    if model:
        meta.append(f"model: {truncate(str(model), 120)}")
    lines.append(f"- {'; '.join(meta)}")

    if users:
        lines.append(f"- first request: {truncate(users[0], 260)}")
    if len(users) > 1:
        lines.append("- user follow-ups:")
        for text in users[1:6]:
            lines.append(f"  - {truncate(text, 220)}")
    if assistant:
        lines.append(f"- final assistant output: {truncate(assistant, 420)}")
    if paths:
        lines.append("- paths mentioned:")
        for path in paths:
            lines.append(f"  - `{truncate(path, 180)}`")
    if not users and not assistant:
        lines.append("- summary: Session ended without supported transcript messages in the hook payload.")

    lines.append("")
    if session_id:
        lines.append(f"<!-- basic-memory-session:{session_id}:end -->")
        lines.append("")

    digest = "\n".join(lines)
    if len(digest) <= MAX_DIGEST_CHARS:
        return digest
    if session_id:
        footer = f"\n<!-- basic-memory-session:{session_id}:end -->\n"
    else:
        footer = "\n"
    return digest[: MAX_DIGEST_CHARS - len(footer) - 2].rstrip() + "…" + footer


def marker_exists(sessions_dir: Path, session_id: str) -> bool:
    marker = f"<!-- basic-memory-session:{session_id}:start -->"
    try:
        paths = sorted({*sessions_dir.glob("*.md"), *sessions_dir.glob("*.markdown")})
    except OSError as exc:
        raise HookError(f"cannot list sessions directory {sessions_dir}: {exc}") from exc
    for path in paths:
        try:
            if marker in path.read_text(encoding="utf-8"):
                return True
        except OSError as exc:
            raise HookError(f"cannot read existing session file {path}: {exc}") from exc
    return False


def append_digest(sessions_dir: Path, started: datetime, digest: str, session_id: str | None) -> tuple[Path, bool]:
    target = sessions_dir / f"{started.strftime('%Y-%m-%d')}.md"
    try:
        with locked_directory(sessions_dir):
            if session_id and marker_exists(sessions_dir, session_id):
                return target, False
            needs_separator = target.exists() and target.stat().st_size > 0
            with target.open("a", encoding="utf-8") as handle:
                if needs_separator:
                    handle.write("\n")
                handle.write(digest.rstrip() + "\n")
    except OSError as exc:
        raise HookError(f"cannot append session digest to {target}: {exc}") from exc
    return target, True


def continuation_json(**extra: Any) -> str:
    payload: dict[str, Any] = {"continue": True, "suppressOutput": True}
    payload.update(extra)
    return json.dumps(payload, sort_keys=True, separators=(",", ":"))


def context_json(context: str) -> str:
    return json.dumps(
        {
            "continue": True,
            "suppressOutput": True,
            "hookSpecificOutput": {
                "hookEventName": "SessionStart",
                "additionalContext": context,
            },
        },
        sort_keys=True,
        separators=(",", ":"),
    )


def session_end(stdin: TextIO = sys.stdin, stdout: TextIO = sys.stdout) -> int:
    try:
        payload = parse_payload(stdin)
        knowledge_dir = knowledge_dir_from_env()
        sessions_dir = ensure_sessions_dir(knowledge_dir)
        messages, transcript_started, transcript_session_id = collect_messages(payload)
        session_id = stable_identifier(payload, transcript_session_id)
        started = digest_started(payload, messages, transcript_started)
        digest = build_digest(payload, messages, started, session_id)
        target, appended = append_digest(sessions_dir, started, digest, session_id)
    except HookError as exc:
        return _error(str(exc))

    if appended:
        message = f"basic memory appended session digest to {target}"
    else:
        message = f"basic memory skipped replayed session {session_id}"
    print(continuation_json(output_file=str(target), systemMessage=message), file=stdout)
    return 0


def session_markdown_files(sessions_dir: Path) -> list[Path]:
    try:
        paths = [path for path in sessions_dir.iterdir() if path.is_file() and path.suffix.lower() in {".md", ".markdown"}]
    except OSError as exc:
        raise HookError(f"cannot list sessions directory {sessions_dir}: {exc}") from exc
    return sorted(paths, key=lambda path: path.name, reverse=True)


def bounded_file_excerpt(path: Path) -> str:
    try:
        text = path.read_text(encoding="utf-8")
    except OSError as exc:
        raise HookError(f"cannot read session file {path}: {exc}") from exc
    text = text.strip()
    if len(text) <= MAX_CONTEXT_FILE_CHARS:
        return text
    return "…\n" + text[-(MAX_CONTEXT_FILE_CHARS - 2) :].lstrip()


def build_context(sessions_dir: Path | None) -> str:
    if sessions_dir is None:
        return ""
    sections: list[str] = []
    total = 0
    for path in session_markdown_files(sessions_dir)[:MAX_CONTEXT_FILES]:
        excerpt = bounded_file_excerpt(path)
        if not excerpt:
            continue
        section = f"### {path.name}\n\n{excerpt}"
        addition = ("\n\n" if sections else "") + section
        if total + len(addition) > MAX_CONTEXT_CHARS:
            remaining = MAX_CONTEXT_CHARS - total
            if remaining > 80:
                sections.append(addition[: remaining - 1].rstrip() + "…")
            break
        sections.append(addition if sections else section)
        total += len(addition)
    if not sections:
        return ""
    context = "Recent session memory digests from $KNOWLEDGE_DIR/sessions:\n\n" + "".join(sections)
    if len(context) > MAX_CONTEXT_CHARS:
        return context[: MAX_CONTEXT_CHARS - 1].rstrip() + "…"
    return context


def session_start(stdin: TextIO = sys.stdin, stdout: TextIO = sys.stdout) -> int:
    try:
        parse_payload(stdin)
        knowledge_dir = knowledge_dir_from_env()
        sessions_dir = require_readable_sessions_dir(knowledge_dir)
        context = build_context(sessions_dir)
    except HookError as exc:
        return _error(str(exc))
    print(context_json(context), file=stdout)
    return 0


DREAM_START_RE = re.compile(r"^<!-- basic-memory-session:(?P<id>[^:]+):start -->$")
DREAM_END_TEMPLATE = "<!-- basic-memory-session:{session_id}:end -->"
DREAM_HEADER_RE = re.compile(
    r"^## Session (?P<captured>\d{4}-\d{2}-\d{2} \d{2}:\d{2} UTC) - (?P<id>.+)$"
)
DREAM_PREFERENCE_RE = re.compile(
    r"^(?:i prefer|i want|please always|always|never|do not|don't)\b",
    re.IGNORECASE,
)
DREAM_CORRECTION_RE = re.compile(
    r"^(?:correction:|actually,|i was wrong\b|i no longer\b|instead,)",
    re.IGNORECASE,
)
LEGACY_SYNC_RE = re.compile(
    r"^## Sync (?P<captured>\d{4}-\d{2}-\d{2} \d{2}:\d{2} UTC)$"
)


@dataclass(frozen=True)
class DreamEvidence:
    path: str
    line_start: int
    line_end: int
    session_id: str
    captured_at: str
    quote: str


@dataclass(frozen=True)
class DreamRecord:
    path: str
    line_start: int
    line_end: int
    session_id: str
    captured_at: str
    block: str
    statements: tuple[DreamEvidence, ...]


def dream_normalize(text: str) -> str:
    normalized = unicodedata.normalize("NFKC", text).casefold()
    normalized = re.sub(r"\s+", " ", normalized).strip()
    return normalized.strip(" \t\r\n.,;:!?\"'()[]{}")


def dream_statement(text: str) -> str | None:
    cleaned = redact_text(text).strip()
    if not cleaned or cleaned.endswith("…"):
        return None
    return cleaned


def parse_dream_record(path: Path, relative_path: str, lines: list[str], start: int, end: int, session_id: str) -> DreamRecord | None:
    header_index = start + 1
    if header_index >= end:
        return None
    header = DREAM_HEADER_RE.match(lines[header_index])
    if not header or header.group("id") != session_id:
        return None
    try:
        captured = datetime.strptime(header.group("captured"), "%Y-%m-%d %H:%M UTC").replace(tzinfo=timezone.utc)
    except ValueError:
        return None

    statements: list[DreamEvidence] = []
    in_followups = False
    for index in range(header_index + 1, end):
        line = lines[index]
        if line.startswith("- first request: "):
            text = dream_statement(line.removeprefix("- first request: "))
            in_followups = False
        elif line == "- user follow-ups:":
            in_followups = True
            continue
        elif in_followups and line.startswith("  - "):
            text = dream_statement(line.removeprefix("  - "))
        else:
            if line.startswith("- "):
                in_followups = False
            continue
        if text is None:
            continue
        statements.append(
            DreamEvidence(
                path=relative_path,
                line_start=index + 1,
                line_end=index + 1,
                session_id=session_id,
                captured_at=captured.isoformat().replace("+00:00", "Z"),
                quote=text,
            )
        )

    block = "\n".join(line.rstrip() for line in lines[start : end + 1])
    return DreamRecord(
        path=relative_path,
        line_start=start + 1,
        line_end=end + 1,
        session_id=session_id,
        captured_at=captured.isoformat().replace("+00:00", "Z"),
        block=block,
        statements=tuple(statements),
    )


def parse_dream_records(path: Path, knowledge_dir: Path) -> list[DreamRecord]:
    try:
        lines = path.read_text(encoding="utf-8").splitlines()
    except OSError as exc:
        raise HookError(f"cannot read session file {path}: {exc}") from exc
    relative_path = str(path.relative_to(knowledge_dir))
    records: list[DreamRecord] = []
    index = 0
    while index < len(lines):
        start_match = DREAM_START_RE.match(lines[index])
        if not start_match:
            index += 1
            continue
        session_id = start_match.group("id")
        end_marker = DREAM_END_TEMPLATE.format(session_id=session_id)
        end = index + 1
        while end < len(lines) and lines[end] != end_marker:
            end += 1
        if end >= len(lines):
            index += 1
            continue
        record = parse_dream_record(path, relative_path, lines, index, end, session_id)
        if record is not None:
            records.append(record)
        index = end + 1
    return records


def dream_evidence_dict(evidence: DreamEvidence) -> dict[str, Any]:
    return {
        "path": evidence.path,
        "line_start": evidence.line_start,
        "line_end": evidence.line_end,
        "session_id": evidence.session_id,
        "captured_at": evidence.captured_at,
        "quote": redact_text(evidence.quote),
    }


def dream_candidate_id(kind: str, normalized_text: str, evidence: list[dict[str, Any]]) -> str:
    coordinates = [
        [item["path"], item["line_start"], item["line_end"], item["session_id"]]
        for item in evidence
    ]
    payload = json.dumps([kind, normalized_text, coordinates], ensure_ascii=False, separators=(",", ":"))
    return "sha256:" + hashlib.sha256(payload.encode("utf-8")).hexdigest()


def repeated_dream_candidates(records: list[DreamRecord]) -> list[dict[str, Any]]:
    grouped: dict[tuple[str, str], list[DreamEvidence]] = {}
    for record in records:
        for statement in record.statements:
            if DREAM_PREFERENCE_RE.match(statement.quote):
                kind = "repeated_preference"
            elif DREAM_CORRECTION_RE.match(statement.quote):
                kind = "repeated_correction"
            else:
                continue
            normalized = dream_normalize(statement.quote)
            grouped.setdefault((kind, normalized), []).append(statement)

    candidates: list[dict[str, Any]] = []
    for (kind, normalized), occurrences in sorted(grouped.items()):
        by_session: dict[str, DreamEvidence] = {}
        for occurrence in sorted(
            occurrences,
            key=lambda item: (item.path, item.line_start, item.session_id),
        ):
            by_session.setdefault(occurrence.session_id, occurrence)
        if len(by_session) < 2:
            continue
        evidence = [dream_evidence_dict(item) for item in by_session.values()]
        action = "promote_for_review"
        candidates.append(
            {
                "id": dream_candidate_id(kind, normalized, evidence),
                "kind": kind,
                "status": "proposed",
                "normalized_text": normalized,
                "reason": f"Exact explicit statement appears in {len(evidence)} distinct sessions.",
                "proposed_action": action,
                "evidence": evidence,
            }
        )
    return candidates


def record_evidence(record: DreamRecord) -> dict[str, Any]:
    return {
        "path": record.path,
        "line_start": record.line_start,
        "line_end": record.line_end,
        "session_id": record.session_id,
        "captured_at": record.captured_at,
        "quote": redact_text(record.block),
    }


def stale_dream_candidates(records: list[DreamRecord]) -> list[dict[str, Any]]:
    grouped: dict[str, list[DreamRecord]] = {}
    for record in records:
        grouped.setdefault(record.session_id, []).append(record)

    candidates: list[dict[str, Any]] = []
    for session_id, occurrences in sorted(grouped.items()):
        if len(occurrences) < 2:
            continue
        ordered = sorted(occurrences, key=lambda item: (item.path, item.line_start))
        evidence = [record_evidence(item) for item in ordered]
        blocks = {item.block for item in ordered}
        expected_date = ordered[0].captured_at[:10]
        canonical = [item for item in ordered if Path(item.path).name == f"{expected_date}.md"]
        if len(blocks) == 1 and len(canonical) == 1:
            kind = "stale_duplicate_record"
            reason = "Byte-equivalent session block exists outside its unique UTC-dated canonical file."
            action = "consolidate_for_review"
            normalized = session_id
        else:
            kind = "record_conflict"
            reason = "Repeated session ID has differing content or no unique UTC-dated canonical file."
            action = "resolve_conflict"
            normalized = session_id
        candidates.append(
            {
                "id": dream_candidate_id(kind, normalized, evidence),
                "kind": kind,
                "status": "proposed",
                "normalized_text": normalized,
                "reason": reason,
                "proposed_action": action,
                "evidence": evidence,
            }
        )
    return candidates


def legacy_dream_candidates(path: Path, knowledge_dir: Path) -> tuple[list[dict[str, Any]], int]:
    try:
        lines = path.read_text(encoding="utf-8").splitlines()
    except OSError as exc:
        raise HookError(f"cannot read legacy memory file {path}: {exc}") from exc
    relative_path = str(path.relative_to(knowledge_dir))
    grouped: dict[str, list[DreamEvidence]] = {}
    section_line = 0
    section_id = ""
    captured_at = ""
    section_count = 0
    for index, line in enumerate(lines):
        heading = LEGACY_SYNC_RE.match(line)
        if heading:
            section_line = index + 1
            section_id = f"legacy:{relative_path}:{section_line}"
            captured = datetime.strptime(heading.group("captured"), "%Y-%m-%d %H:%M UTC").replace(tzinfo=timezone.utc)
            captured_at = captured.isoformat().replace("+00:00", "Z")
            section_count += 1
            continue
        if not section_id or not line.startswith("- "):
            continue
        quote = dream_statement(line.removeprefix("- "))
        if quote is None:
            continue
        normalized = dream_normalize(quote)
        if not normalized:
            continue
        grouped.setdefault(normalized, []).append(
            DreamEvidence(
                path=relative_path,
                line_start=index + 1,
                line_end=index + 1,
                session_id=section_id,
                captured_at=captured_at,
                quote=quote,
            )
        )

    candidates: list[dict[str, Any]] = []
    for normalized, occurrences in sorted(grouped.items()):
        by_section: dict[str, DreamEvidence] = {}
        for occurrence in occurrences:
            by_section.setdefault(occurrence.session_id, occurrence)
        if len(by_section) < 2:
            continue
        all_evidence = [dream_evidence_dict(item) for item in by_section.values()]
        shown_evidence = all_evidence[:20]
        candidates.append(
            {
                "id": dream_candidate_id("stale_duplicate_record", normalized, all_evidence),
                "kind": "stale_duplicate_record",
                "status": "proposed",
                "normalized_text": normalized,
                "reason": f"Exact legacy fact appears in {len(all_evidence)} distinct sync records.",
                "proposed_action": "consolidate_for_review",
                "occurrence_count": len(all_evidence),
                "evidence": shown_evidence,
                "evidence_omitted": len(all_evidence) - len(shown_evidence),
            }
        )
    return candidates, section_count


def build_dream_report(knowledge_dir: Path) -> dict[str, Any]:
    sessions_dir = require_readable_sessions_dir(knowledge_dir)
    files = session_markdown_files(sessions_dir) if sessions_dir is not None else []
    records: list[DreamRecord] = []
    for path in sorted(files, key=lambda item: item.name):
        records.extend(parse_dream_records(path, knowledge_dir))
    records.sort(key=lambda item: (item.path, item.line_start, item.session_id))

    legacy_candidates: list[dict[str, Any]] = []
    legacy_records = 0
    if sessions_dir is not None:
        legacy_path = sessions_dir / "knowledge.md"
        if legacy_path.is_file():
            legacy_candidates, legacy_records = legacy_dream_candidates(legacy_path, knowledge_dir)
    candidates = repeated_dream_candidates(records) + stale_dream_candidates(records) + legacy_candidates
    candidates.sort(key=lambda item: (item["kind"], item["id"]))
    counts = {
        "repeated_preferences": sum(item["kind"] == "repeated_preference" for item in candidates),
        "repeated_corrections": sum(item["kind"] == "repeated_correction" for item in candidates),
        "stale_records": sum(item["kind"] == "stale_duplicate_record" for item in candidates),
        "conflicts": sum(item["kind"] == "record_conflict" for item in candidates),
    }
    return {
        "schema_version": 1,
        "mode": "review-only",
        "source": {
            "root": "$KNOWLEDGE_DIR",
            "patterns": ["sessions/*.md", "sessions/*.markdown"],
            "files_scanned": len(files),
            "records_scanned": len(records) + legacy_records,
        },
        "summary": counts,
        "candidates": candidates,
    }


def write_dream_report(knowledge_dir: Path, output: Path, report: dict[str, Any]) -> None:
    reviews_dir = (knowledge_dir / "reviews").resolve()
    resolved = output.expanduser().resolve()
    if not resolved.is_relative_to(reviews_dir):
        raise HookError(f"dream output must be under {reviews_dir}")
    if resolved.exists():
        raise HookError(f"dream output already exists: {resolved}")
    resolved.parent.mkdir(parents=True, exist_ok=True)
    content = json.dumps(report, ensure_ascii=False, indent=2, sort_keys=True) + "\n"
    fd, tmp_name = tempfile.mkstemp(prefix=f".{resolved.name}.", suffix=".tmp", dir=str(resolved.parent))
    tmp_path = Path(tmp_name)
    try:
        with os.fdopen(fd, "w", encoding="utf-8") as handle:
            handle.write(content)
            handle.flush()
            os.fsync(handle.fileno())
        os.link(tmp_path, resolved)
    except FileExistsError as exc:
        raise HookError(f"dream output already exists: {resolved}") from exc
    except OSError as exc:
        raise HookError(f"cannot write dream report {resolved}: {exc}") from exc
    finally:
        try:
            tmp_path.unlink()
        except OSError:
            pass


def dream_command(argv: list[str], stdout: TextIO = sys.stdout) -> int:
    parser = argparse.ArgumentParser(prog="basic_memory.py dream")
    parser.add_argument("--output", required=True, type=Path)
    args = parser.parse_args(argv)
    try:
        knowledge_dir = knowledge_dir_from_env()
        report = build_dream_report(knowledge_dir)
        write_dream_report(knowledge_dir, args.output, report)
    except HookError as exc:
        return _error(str(exc))
    counts = report["summary"]
    print(
        json.dumps(
            {
                "output_file": str(args.output.expanduser().resolve()),
                "mode": "review-only",
                "summary": counts,
            },
            sort_keys=True,
            separators=(",", ":"),
        ),
        file=stdout,
    )
    return 0


def main(argv: list[str] | None = None) -> int:
    args = sys.argv[1:] if argv is None else argv
    if not args or args[0] != "dream":
        print("usage: basic_memory.py dream --output $KNOWLEDGE_DIR/reviews/<name>.json", file=sys.stderr)
        return 2
    return dream_command(args[1:])


if __name__ == "__main__":
    raise SystemExit(main())
