# bin/memsearch

Local tooling that wraps the upstream memsearch Claude Code plugin so it writes into a single global vault and shared Milvus collection.

## hook.sh

Recursion-safe wrapper invoked by the Claude Code SessionStart, Stop, and SessionEnd hooks. Sets `MEMSEARCH_MEMORY_DIR`, `MEMSEARCH_STATE_DIR`, and `MEMSEARCH_COLLECTION_NAME` so memsearch operates on `~/Documents/knowledge/ai` with collection `ai` regardless of cwd. Then execs the upstream plugin hook for the event.

The `MEMSEARCH_SKIP_CLAUDE_HOOKS=1` guard prevents the `claude -p` child spawned by `stop.sh` from recursively re-entering hooks and fork-bombing.

## apply-local.sh

Idempotent patcher that rewrites four files under `~/Public/memsearch/plugins/claude-code/` so the upstream plugin honors our env var overrides and writes per-turn memory entries with our preferred `## HH:MM claude-code` heading format. Run it after any `git pull` inside `~/Public/memsearch`. Safe to run twice.

## backfill/

One-time Go tool that consolidates historical Claude Code and Codex session transcripts into daily digest files in the vault. Kept here as an artifact in case re-consolidation is ever needed. NOT part of the live pipeline. Build with `go build .` inside this directory. The compiled binary is gitignored.
