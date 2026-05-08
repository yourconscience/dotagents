# memory

Agent-agnostic session memory for dotagents.

This directory is designed to make AI session context durable across tools
without tying the repo layout to any one agent or memory provider. Hook
entrypoints capture compact session digests, sync selected memory facts into the
knowledge vault, and keep the derived search index current.

`memory/` is the stable abstraction. `memsearch` is the current indexing
provider and should remain an implementation detail behind configuration and
library code.

## Setup

```bash
uv tool install memsearch
dotagents memsearch setup --vault ~/Workspace/knowledge
```

This creates the vault directories, initializes the vault as a git repo if
needed, and writes the local memory-search configuration.

## Design

- Keep lifecycle concepts generic: `session-start`, `stop`, `session-end`, and
  sync.
- Avoid per-agent directories unless an integration boundary forces them.
- Prefer payload dispatch and small parser modules over duplicated hook
  implementations.
- Index durable summaries and curated vault content, not raw full transcripts.
- Treat the search index as derived state; Markdown in the knowledge vault is
  canonical.

## Layout

- `hooks/`: executable lifecycle hook entrypoints.
- `lib/`: transcript digest, vault sync, and indexing helpers.
- `AGENTS.md`: local rules for keeping this area agent-agnostic.

## Hook registration

Agent config patching and migration should live in the dotagents CLI, not in
copy-pasted README snippets. Use `dotagents memsearch setup` for local
configuration and keep manual hook details in the dedicated troubleshooting
reference under `skills/dotagents/references/`.
