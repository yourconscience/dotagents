# Memory provider evaluation harness

Privacy-safe, isolated comparison of Hermes memory providers against the
current dotagents baseline (built-in memory + memsearch). No adapter touches
the live Hermes home, the `ai` memsearch collection, or the knowledge vault.

## Components

| Path | Purpose |
| --- | --- |
| `evaluate.py` | CLI: `privacy-scan`, `capabilities`, `run` |
| `testdata/v1/` | Frozen synthetic fixture (12 docs, 18 queries, 3 mutations, 3 conversations) |
| `tests/` | Unit tests for fixture validation, scoring, adapters, runner, discovery |

## Fixture provenance

Fully synthetic. Technical themes only (memory tiers, hook contracts, sync
semantics, redaction, dedup classification). No source documents were copied.
People are codenames (Atlas/Bramble/Cinder); the only hostname uses `.test`
(RFC 2606) and the only absolute path is under `/srv/fixture`.
`fixture-secret-hook-0001` is a deliberately inert canary used by conversation
checks, not a credential.

The fixture has passed the automated privacy scan (zero findings) but carries
`approval.state: pending_human_review` in `privacy_manifest.json` until you
explicitly approve it. `run --approved-fixture` records the human gate in the
result payload; it does not change fixture contents.

## Usage

```bash
# Validate fixture and scan for private data (exit 1 on any finding)
python3 memory/eval/evaluate.py privacy-scan --fixture memory/eval/testdata/v1

# Inventory installed provider plugins + runtime availability
python3 memory/eval/evaluate.py capabilities --output /tmp/capabilities.json

# Run both baselines plus all known providers (unconfigured ones report capability_gap)
python3 memory/eval/evaluate.py run --fixture memory/eval/testdata/v1 \
  --provider all --output /tmp/results.json

# Optional: local canaries file (newline-delimited; values never printed)
python3 memory/eval/evaluate.py privacy-scan --fixture memory/eval/testdata/v1 \
  --forbidden-file ~/.hermes/cache/eval-canaries.txt
```

## Guarantees

- Every adapter is rooted under a per-run temporary directory; memsearch arms
  use throwaway `dotagents_eval_<hex>` collections and `teardown()` resets
  them, so live data and the `ai` collection are never touched.
- Privacy scan gates `run`: a nonzero finding count aborts before any ingest.
- Failed/absent providers emit `capability_gap` or `failed` with reasons —
  never a zero-quality score.
- Results record the fixture hash, environment, per-query rankings, latencies,
  lifecycle outcomes, and capability gaps under a versioned schema.

## Adding a real provider driver

Implement `ProviderAdapter` (`health`, `reset`, `ingest`, `query`, `update`,
`forget`, `restart`, `export`, `capture`, `teardown`) in `evaluate.py` (or a
sibling module) and register it in `make_adapter`. The runner, scorer, and
privacy gate apply automatically. Record unsupported operations explicitly;
the plan forbids scoring gaps as zeros.

## v1 baseline numbers (single-run, directional)

From the isolated run on this machine (fixture hash
`ff1182fc8fd32d83…`, memsearch collection `dotagents_eval_<random>`):

| Provider | Recall@1 | Recall@3 | MRR | Abstention | Query p50 |
| --- | --- | --- | --- | --- | --- |
| memsearch | 0.778 | 0.833 | 0.806 | 0.0 | ~1.8 s |
| built-in | 0.056 | 0.111 | 0.099 | 0.0 | <1 ms |

Abstention is 0.0 for both because pure retrieval never refuses to answer —
an LLM judge or thresholded re-ranker is required to act on unanswerable
queries; the harness records the gap rather than hiding it.
