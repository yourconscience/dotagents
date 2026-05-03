#!/usr/bin/env python3
"""Sync Hermes built-in memory with the dotagents knowledge vault.

Direction 1 (memory→vault): export Hermes MEMORY.md/USER.md facts to vault
  - Hermes MEMORY.md → vault ai/knowledge.md
  - Hermes USER.md → vault profile/USER.md (appended as "Hermes Memory Sync" section)

Direction 2 (vault→memory): import vault profile facts into Hermes memory
  - vault profile/USER.md → Hermes USER.md (compact, deduplicated)
  - vault recent ai/ facts → Hermes MEMORY.md (compact, deduplicated)

After either direction, reindexes memsearch.

Usage:
  python sync.py memory-to-vault   # export Hermes → vault
  python sync.py vault-to-memory   # import vault → Hermes
  python sync.py both              # bidirectional
"""

import os
import re
import sys
from datetime import datetime, timezone
from pathlib import Path


# ── Paths ─────────────────────────────────────────────────────────────
def resolve():
    home = Path(os.path.expanduser("~"))
    return {
        "hermes_memory": home / ".hermes" / "memories" / "MEMORY.md",
        "hermes_user": home / ".hermes" / "memories" / "USER.md",
        "vault_profile": home / "Workspace" / "knowledge" / "profile" / "USER.md",
        "vault_knowledge": home / "Workspace" / "knowledge" / "ai" / "knowledge.md",
        "vault_ai_dir": home / "Workspace" / "knowledge" / "ai",
        "vault_notes_dir": home / "Workspace" / "knowledge" / "notes",
        "vault_profile_dir": home / "Workspace" / "knowledge" / "profile",
        "collection": "ai",
    }


# ── Parse Hermes §-delimited memory ──────────────────────────────────
def parse_hermes_memory(path: Path) -> list[str]:
    """Split Hermes memory file by § delimiter, return non-empty entries."""
    if not path.exists():
        return []
    text = path.read_text(encoding="utf-8").strip()
    if not text:
        return []
    entries = [e.strip() for e in text.split("§")]
    return [e for e in entries if e]


def normalize(entry: str) -> str:
    """Normalize for dedup comparison: lowercase, drop punctuation, collapse whitespace."""
    text = entry.lower().strip()
    text = re.sub(r"[^\w+]+", " ", text)
    return re.sub(r"\s+", " ", text).strip()


# ── Direction 1: Memory → Vault ──────────────────────────────────────
def memory_to_vault(paths: dict):
    """Export Hermes memory facts to the knowledge vault."""
    changed = False

    # --- MEMORY.md → ai/knowledge.md ---
    memory_entries = parse_hermes_memory(paths["hermes_memory"])
    knowledge_path = paths["vault_knowledge"]

    existing_knowledge = set()
    if knowledge_path.exists():
        for line in knowledge_path.read_text(encoding="utf-8").splitlines():
            line = line.strip().lstrip("- ").strip()
            if line:
                existing_knowledge.add(normalize(line))

    new_entries = []
    for entry in memory_entries:
        if normalize(entry) not in existing_knowledge:
            new_entries.append(entry)

    if new_entries:
        knowledge_path.parent.mkdir(parents=True, exist_ok=True)
        now = datetime.now(timezone.utc).strftime("%Y-%m-%d %H:%M UTC")
        with knowledge_path.open("a", encoding="utf-8") as f:
            if not knowledge_path.exists() or knowledge_path.stat().st_size == 0:
                f.write("# Hermes Memory Export\n\n")
                f.write(f"Auto-synced from Hermes built-in memory. Last sync: {now}\n\n")
            else:
                f.write(f"\n## Sync {now}\n\n")
            for entry in new_entries:
                f.write(f"- {entry}\n")
        print(f"memory→vault: wrote {len(new_entries)} new entries to {knowledge_path}")
        changed = True
    else:
        print(f"memory→vault: {knowledge_path} up to date (no new entries)")

    # --- USER.md → profile/USER.md ---
    user_entries = parse_hermes_memory(paths["hermes_user"])
    profile_path = paths["vault_profile"]

    if profile_path.exists():
        profile_text = profile_path.read_text(encoding="utf-8")
    else:
        profile_text = ""

    marker = "## Hermes Memory Sync"
    base_profile_text = profile_text[:profile_text.index(marker)].rstrip() if marker in profile_text else profile_text.rstrip()

    now = datetime.now(timezone.utc).strftime("%Y-%m-%d %H:%M UTC")
    sync_section = f"\n\n{marker}\n\nLast synced: {now}\n\n"
    for entry in user_entries:
        sync_section += f"- {entry}\n"
    sync_section += "\n"

    current_section_body = ""
    if marker in profile_text:
        current_section_body = profile_text[profile_text.index(marker):].strip()

    desired_section_body = sync_section.strip()
    # Ignore timestamp when deciding idempotence.
    current_without_timestamp = re.sub(r"Last synced: .*", "Last synced: <timestamp>", current_section_body)
    desired_without_timestamp = re.sub(r"Last synced: .*", "Last synced: <timestamp>", desired_section_body)

    if current_without_timestamp != desired_without_timestamp:
        profile_path.write_text(base_profile_text + sync_section, encoding="utf-8")
        print(f"memory→vault: wrote {len(user_entries)} user facts to {profile_path}")
        changed = True
    else:
        print(f"memory→vault: {profile_path} up to date (no user fact changes)")

    return changed


# ── Direction 2: Vault → Memory ──────────────────────────────────────
# Hermes char limits for memory files
HERMES_MEMORY_LIMIT = 2200
HERMES_USER_LIMIT = 1375


def vault_to_memory(paths: dict):
    """Import vault facts into Hermes built-in memory, respecting char limits."""
    changed = False

    # --- profile/USER.md → Hermes USER.md ---
    profile_path = paths["vault_profile"]
    hermes_user_path = paths["hermes_user"]

    existing_user = {normalize(e) for e in parse_hermes_memory(hermes_user_path)}

    # Extract bullet points from profile sections — full facts, no truncation
    profile_facts = []
    if profile_path.exists():
        for line in profile_path.read_text(encoding="utf-8").splitlines():
            stripped = line.strip()
            if stripped.startswith("- "):
                fact = stripped[2:].strip()
                # Skip trivial bullets, skip "Hermes Memory Sync" section
                if fact and len(fact) > 15:
                    profile_facts.append(fact)

    # Deduplicate against existing Hermes user facts. Consolidated Hermes facts
    # often contain several shorter vault bullets, so treat substring containment
    # and significant-token subset matches as duplicates too, not just exact
    # normalized equality.
    def tokens(text: str) -> set[str]:
        stop = {"a", "an", "and", "the", "with", "in", "on", "at", "from", "via", "to", "of"}
        return {t for t in normalize(text).split() if t not in stop}

    existing_tokens = [tokens(existing) for existing in existing_user]
    new_facts = []
    for fact in profile_facts:
        fact_norm = normalize(fact)
        fact_tokens = tokens(fact)
        if fact_norm in existing_user:
            continue
        if any(fact_norm in existing or existing in fact_norm for existing in existing_user):
            continue
        if len(fact_tokens) >= 2 and any(fact_tokens <= toks for toks in existing_tokens):
            continue
        if len(fact_tokens) >= 2 and any(len(fact_tokens & toks) / len(fact_tokens) >= 0.70 for toks in existing_tokens):
            continue
        new_facts.append(fact)

    if new_facts:
        existing_entries = parse_hermes_memory(hermes_user_path)
        # Build new USER.md: existing facts first (priority), then new vault facts within limit
        lines = list(existing_entries)

        def rendered_size(entries: list[str]) -> int:
            if not entries:
                return 0
            return len("\n§\n".join(entries) + "\n")

        current_size = rendered_size(lines)

        for fact in new_facts:
            candidate = lines + [fact]
            candidate_size = rendered_size(candidate)
            if candidate_size <= HERMES_USER_LIMIT:
                lines.append(fact)
                current_size = candidate_size
            else:
                break

        hermes_user_path.write_text(
            "\n§\n".join(lines) + "\n",
            encoding="utf-8",
        )
        added = len(lines) - len(existing_entries)
        print(f"vault→memory: added {added} facts to {hermes_user_path} ({current_size}/{HERMES_USER_LIMIT} chars)")
        changed = True
    else:
        print(f"vault→memory: {hermes_user_path} up to date")

    return changed


# ── Reindex ──────────────────────────────────────────────────────────
def reindex_memsearch(paths: dict):
    """Reindex the memsearch collection after changes."""
    import subprocess
    cmd = [
        "memsearch", "index",
        str(paths["vault_notes_dir"]),
        str(paths["vault_profile_dir"]),
        str(paths["vault_ai_dir"]),
        "--collection", paths["collection"],
    ]
    result = subprocess.run(cmd, capture_output=True, text=True)
    if result.returncode == 0:
        print("memsearch reindex: done")
    else:
        print(f"memsearch reindex: warning — {result.stderr.strip()}")


# ── Main ─────────────────────────────────────────────────────────────
def main():
    if len(sys.argv) < 2:
        print("Usage: sync.py [memory-to-vault|vault-to-memory|both]", file=sys.stderr)
        sys.exit(1)

    paths = resolve()
    direction = sys.argv[1]
    any_change = False

    if direction in ("memory-to-vault", "both"):
        any_change |= memory_to_vault(paths)

    if direction in ("vault-to-memory", "both"):
        any_change |= vault_to_memory(paths)

    if any_change:
        reindex_memsearch(paths)


if __name__ == "__main__":
    main()
