# memory

Agent-agnostic session memory and private data sync for dotagents.

This directory is the **canonical upstream** of the memory layer: hook
entrypoints capture compact session digests, `rem` captures explicit facts,
consolidation is report-first, and `knowledge-sync` keeps the knowledge vault
git-synced. User repos deployed via `dotagents setup` carry their own copy of
`memory/lib`, `memory/hooks`, and `memory/tools`; `dotagents sync` builds the
tools and this repo carries the tested reference implementations.

## The rem workflow

```bash
rem add -src claude "prefers pnpm for Node work"   # capture a candidate fact anywhere
rem dream                                          # consolidation report (report-only)
rem dream --apply                                  # collapse exact-duplicate records
rem search "quota preferences"                     # semantic search via memsearch
rem sync                                           # flush the vault via knowledge-sync
```

Candidates are inert until promoted into durable instructions — consolidation
is report-first by design. See [tools/rem/README.md](tools/rem/README.md).

## Setup

Choose a tier through the CLI:

```bash
dotagents setup --memory basic      # default; Python 3 only
dotagents setup --memory off        # no managed memory hooks
dotagents setup --memory memsearch  # indexed search; requires memsearch on PATH
```

| Tier | Behavior | Dependency |
|---|---|---|
| `off` | no managed memory hooks | none |
| `basic` | bounded session digests into the knowledge vault | Python 3 |
| `memsearch` | adds a derived search index over the vault | `memsearch` |

## Layout

- `hooks/` — lifecycle entrypoints (session start/end/stop) registered per harness
- `lib/` — Python implementation: `basic_memory.py` (digests, dream-pass parsing),
  `sync.py` (Hermes memory ↔ vault), `safety.py`
- `tools/` — Go binaries built by `dotagents sync`: `rem`, `knowledge-sync`
- `tests/` — reference test suite

## Relationship to user repos

`dotagents` (this repo) is upstream: changes land here first, with tests. Your
`~/.agents` repository (created by `setup`) carries the deployed copy; `sync`
rebuilds the binaries whenever the sources change. Keep the two in sync by
porting changes here, then pulling in the user repo.
