# memory

Agent-agnostic session memory and private data sync for dotagents.

This directory makes AI session context and private skill data durable across
tools and machines without tying the layout to any one agent or memory provider.
Hook entrypoints capture compact session digests, sync selected memory facts into
the knowledge vault, and keep the derived search index current.

`memory/` is the stable abstraction. `memsearch` is the current indexing
provider and should remain an implementation detail behind configuration and
library code.

## Setup

```bash
uv tool install memsearch==0.4.2
dotagents setup memsearch --vault "$HOME/knowledge"
```

This creates the vault directories, initializes the vault as a git repo if
needed, and writes the local configuration.

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

Agent config patching and migration should live in the dotagents CLI, not in
copy-pasted README snippets. Use `dotagents setup memsearch` for local
configuration and keep manual hook details in the dedicated troubleshooting
reference under `skills/dotagents/references/`.
