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
each other:

- **Command:** `memsearch index "$KNOWLEDGE_DIR" --collection ai` (incremental;
  only changed files are re-embedded).
- **Idempotent:** re-running is safe; markdown is canonical.
- **Non-overlapping lock:** an OS advisory lock (`flock`/`fcntl.flock`,
  non-blocking) on `"$MEMSEARCH_HOME/reindex.lock"` (default
  `~/.memsearch/reindex.lock`). If the lock is already held, the trigger skips;
  the next trigger catches up.
- **Best-effort:** a refresh never blocks or fails its caller. `knowledge-sync`
  additionally bounds it with a timeout (`MEMSEARCH_REINDEX_TIMEOUT_SECONDS`,
  default 300s).
- **No-op without the engine:** on a sync-only node where `memsearch` is not on
  `PATH`, the trigger silently does nothing.
