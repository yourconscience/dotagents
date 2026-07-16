# memory

Agent-agnostic session memory and private data sync for dotagents.

This directory makes AI session context and private skill data durable across
tools and machines without tying the layout to any one agent or memory provider.
Hook entrypoints capture compact session digests, sync selected memory facts into
the knowledge vault, and keep the derived search index current.

`memory/` is the stable abstraction. The dependency-free basic tier writes
bounded Markdown digests directly; `memsearch` remains an optional indexed
provider behind the full hook pipeline.

## Setup

Choose a tier through the CLI:

```bash
dotagents setup --memory basic      # default; Python 3 only
dotagents setup --memory off        # no managed memory hooks
dotagents setup --memory memsearch  # requires memsearch on PATH
```

Basic memory appends compact digests under
`$KNOWLEDGE_DIR/sessions/YYYY-MM-DD.md` and injects bounded recent context at
session start. It does not invoke memsearch or persist raw transcripts.

The memsearch tier registers the indexed SessionStart, Stop, and SessionEnd
pipeline. Configure its vault through `~/.agents/memsearch.conf`.

## Canonical paths

The knowledge vault path is set in `~/.agents/memsearch.conf` as `KNOWLEDGE_DIR`.
All skills and hooks resolve data paths relative to this variable:

```
$KNOWLEDGE_DIR/              # vault root (default: ~/Workspace/knowledge)
  sessions/                  # dated session digests (YYYY-MM-DD.md)
  notes/                     # handwritten notes
  profile/                   # USER.md, WORK.md
  skills/<name>/             # private data for shared skills
```

Do not hardcode `~/Workspace/knowledge` in code. Use `$KNOWLEDGE_DIR`.

Skills with private data store it under `$KNOWLEDGE_DIR/skills/<skill-name>/`.
Example: `$KNOWLEDGE_DIR/skills/jobs/opportunities.yaml`.

## Design principles

- No symlinks. One canonical location per data file.
- Reuse existing sync pipelines when generalizing. Do not add ad-hoc rsync,
  cron, or new sync tooling for individual skills.
- The knowledge vault is the canonical store for all private data that needs
  to be available across agents and machines.
- The search index (memsearch) is derived state. Markdown in the vault is
  canonical.
- Keep lifecycle concepts generic: `session-start`, `stop`, `session-end`, sync.
- Avoid per-agent directories unless an integration boundary forces them.
- Prefer payload dispatch and small parser modules over duplicated hooks.
- Index durable summaries and curated vault content, not raw full transcripts.

## Security

The knowledge vault contains private data. It must not be pushed to any public
remote. Distribution is limited to local git and private sync between trusted
machines (Mac and VPS via the knowledge-sync tool).

## Layout

- `hooks/`: executable lifecycle hook entrypoints.
- `lib/`: transcript digest, vault sync, and indexing helpers.
- `tools/`: small repo-owned operational helpers for the memory subsystem.
- `AGENTS.md`: local rules for keeping this area agent-agnostic.

## Hook registration

Memory-tier selection and native hook registration live in the dotagents CLI,
not in copy-pasted setup snippets. Re-run `dotagents setup --memory <tier>` to
change the managed hooks. Keep manual troubleshooting details in
`skills/dotagents/references/memory-sync.md`.
