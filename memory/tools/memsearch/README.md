# memsearch parity

`memsearch` is the semantic index/search engine over the knowledge vault. It is
a first-class, equal-citizen backend on every machine: each host runs the same
engine with the same config and builds its own **derived** index from the shared
canonical markdown.

- **Markdown is the only source of truth.** The per-machine index
  (`~/.memsearch/milvus.db`) is a disposable derivative. It is never synced and
  never canonical -- only the vault markdown is. Any machine can rebuild its
  index from zero at any time.
- **Collection:** `ai`.
- **Embeddings:** local ONNX (`provider = onnx`), no API keys, no external cost.

## Reach parity on a new machine

```bash
# one command: install engine, write config, build the index, verify
memory/tools/memsearch/install-parity.sh
```

It is idempotent, so it is safe to re-run. See `install-parity.sh --help` for
flags (`--rebuild`, `--no-verify`) and the `KNOWLEDGE_DIR` /
`MEMSEARCH_COLLECTION` / `MEMSEARCH_HOME` overrides.

What it does, equivalently by hand:

```bash
uv tool install 'memsearch[onnx]'                 # local ONNX embeddings
memsearch config set embedding.provider onnx
memsearch config set milvus.collection ai
memsearch index ~/Workspace/knowledge --collection ai
memsearch search "knowledge vault memory" --collection ai   # verify
```

## Full rebuild from zero

The index is disposable; rebuild it whenever it drifts or a machine is new:

```bash
memory/tools/memsearch/install-parity.sh --rebuild
# equivalently:
memsearch reset --collection ai --yes        # drop the collection
memsearch index ~/Workspace/knowledge --collection ai
```

## Keeping the index fresh (no cron)

There is deliberately **no reindex cron**. Two idempotent triggers keep the
derived index current, both refreshing the whole vault into collection `ai`:

| Trigger | Owner | When |
|---|---|---|
| reindex-after-sync | `knowledge-sync` (this repo) | after each vault pull/merge/push |
| reindex-after-capture | `basic_memory` capture path | after a session digest is written |

### Shared reindex contract

Both triggers honor the same contract so they stay coordinated and never step on
each other (canonical definition: `memory/hooks/common.sh` `refresh_index_async`,
mirrored by `knowledge-sync`):

- **Scope:** the notes and profile directories plus every `sessions/*.md` and
  `*.markdown` file, indexed into collection `ai` (incremental; only changed
  files are re-embedded). Paths resolve from `NOTES_DIR` / `PROFILE_DIR` /
  `SESSIONS_DIR`, defaulting under `KNOWLEDGE_DIR`.
- **Idempotent:** re-running is safe; markdown is canonical. `ai` is pinned;
  `MEMSEARCH_COLLECTION` drift is ignored so `ai` cannot go stale.
- **Non-overlapping lock:** an atomic **`mkdir` lock** at
  `"${MEMSEARCH_STATE_DIR:-~/.memsearch/state}/reindex.lock"` (a directory) with
  the owner pid written to `reindex.lock/pid`. `mkdir` is atomic, so a second
  refresh fails to create it and skips; a lock whose recorded owner is no longer
  alive (`kill -0`) is reclaimed so a killed refresher cannot suppress reindex
  forever. Both triggers use this exact path and mechanism.
- **Best-effort:** a refresh never blocks or fails its caller, bounded by a
  timeout (`MEMSEARCH_REINDEX_TIMEOUT`, seconds, default 120).
- **No-op without the engine:** on a sync-only node where `memsearch` is not on
  `PATH`, the trigger silently does nothing.
