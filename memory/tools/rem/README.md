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

Environment:

- `KNOWLEDGE_DIR` - vault root (default `~/Workspace/knowledge`)
- `REM_SYNC_BIN` - alternate knowledge-sync binary for `rem sync` (default `~/.local/bin/knowledge-sync`)

Design and rationale: `plans/rem-plan-2026-08.md` in the knowledge vault.

Tests: `go test ./...`
