# rem

Memory workflow CLI for the knowledge vault. Shipped as a dotagents memory tool:
`dotagents sync` builds it and installs it to `$GOBIN` or `~/.local/bin`.

```
rem add [-src harness] "<fact>"   capture a candidate fact to $KNOWLEDGE_DIR/ai/YYYY-MM-DD.md
rem search "<query>"              semantic search via memsearch (collection ai)
rem dream [--apply]               consolidation report; --apply collapses exact-duplicate
                                  sync sections in sessions/knowledge.md (backup + commit)
rem sync                          commit+merge+push the vault via knowledge-sync
```

`rem search` targets collection `ai` (the canonical vault collection) unless you
pass your own `-c`/`--collection`.

Environment:

- `KNOWLEDGE_DIR` - vault root (default `~/Workspace/knowledge`)
- `REM_SYNC_BIN` - alternate knowledge-sync binary for `rem sync` (default `~/.local/bin/knowledge-sync`)
- `REM_COLLECTION` - collection for `rem search` (default `ai`)

## Consolidation: one owner per input (R4)

There are two `dream` passes; they consume **different inputs** and neither
feeds the other, so ownership is split deliberately to avoid duplication:

| Pass | Owner | Input | Output |
|---|---|---|---|
| `rem dream` (this tool) | Go | `ai/` candidate captures | promotion report in `reviews/`; `--apply` collapses exact-duplicate sync sections in `sessions/knowledge.md` |
| `basic_memory dream` | Python (`lib/basic_memory.py`) | `sessions/` session digests | review-only JSON in `reviews/` |

`rem dream` owns the explicit `rem add` candidate loop (promotion into
`profile/USER.md` or `AGENTS.md`); `basic_memory dream` owns automatic session
digests. Keep them on separate inputs -- do not point either at the other's
source.

Design and rationale: `plans/rem-plan-2026-08.md` in the knowledge vault.

Tests: `go test ./...`
