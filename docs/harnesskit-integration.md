# HarnessKit integration — design notes

Status: draft / thinking. Branch: `feat/harnesskit-integration`. Date: 2026-09-06.

## Finding

[HarnessKit](https://github.com/RealZST/HarnessKit) (RealZST/HarnessKit, Rust, Apache-2.0, ~420★, active) is a web UI (also desktop/CLI) that inspects and manages agent extensions, configs, memory, and rules across 13 harnesses. Verified live against the maintainer's machine on 2026-09-06: it detects and reads the **full dotagents stack** — Claude Code, Codex, **Oh My Pi** (`~/.omp/agent/`), **Hermes** (`~/.hermes/`), plus Gemini CLI, Copilot, OpenCode, Grok Build. This is exactly the coverage (Pi/OMP + Hermes) that CCO and ai-config-sync-manager lack.

Consequence: dotagents does **not** need to build its own config viewer. HarnessKit already does the read/inspect/audit surface better than we would from scratch, and on every harness we care about.

## Why integrate, and the one boundary rule

HarnessKit and dotagents are complementary, not competing:

- **HarnessKit** = read/inspect/audit dashboard + marketplace. Reads *materialized native dirs*. Its write model is **convergence** ("deploy this extension to every agent").
- **dotagents** = sync engine + source of truth. Manages the 5 surfaces (skills, MCP, hooks, roles, plugins) via symlinks + `dotagents.lock` + intentional per-harness divergence.

**Boundary invariant for this integration:** dotagents stays the only writer. HarnessKit is consumed read-mostly. We never route dotagents' managed surfaces *through* HarnessKit's convergence writer, and we never let HK's "deploy to all" become the mechanism that mutates a dotagents-owned symlink. Divergence is a feature here, not drift — HK's model treats it as drift, so its write path is off-limits for managed surfaces.

## Integration levels

### L0 — Recommend (docs only, zero coupling)

Name HarnessKit in `README.md`, `docs/comparison.md`, and the `dotagents` skill as the inspection dashboard: "dotagents owns sync; use HarnessKit to see/audit the result across harnesses." No code. Ships today.

### L1 — Optional dependency (`deps` / `setup`)

Register HarnessKit as an **optional, opt-in** external tool:

- `dotagents setup` offers (never forces) HK install after the first sync, gated behind a prompt.
- `dotagents deps check` reports whether HK is present + version; `deps update` bumps it.
- Honor the existing publish-age gate (`checkExternalPackageAge`, `package_age.go`) — HK is a fast-moving Rust binary; do not auto-pull a release younger than the configured window.

Install method is an **open question** (see below) — do not hardcode one until verified.

### L2 — Launch command (`dotagents view`)

New subcommand that starts HK pointed at the active dotagents config root and opens the browser:

- Resolves the config root the same way the rest of the CLI does (`--config` → `$DOTAGENTS_HOME` → `~/.agents`).
- Spawns the HK local server (127.0.0.1, token in URL — as observed at `:7070`), prints the URL, optionally opens it. Mirrors the existing external-CLI launch path (`external_cli.go`, `cli_launch_test.go`).
- Read-only intent: we launch HK as a viewer over what dotagents already materialized.

L0–L2 are the concrete near-term scope. All three keep the boundary invariant trivially (no managed-surface writes).

### L3 — Write-through (research spike, do not build yet)

The "can we drive changes from HK's UI via dotagents commands" question. HK writes to native dirs directly; dotagents owns those via symlinks/lock. Reconciling them needs one of:

- **(a) HK-as-frontend:** HK calls `dotagents` as its backend for mutations. Requires HK to expose a pluggable write backend — **not known to exist**; would need an upstream change or fork. Verify before assuming.
- **(b) Watch-and-reconcile:** dotagents watches HK's writes and folds them back into the canonical store + re-syncs. Fragile (race with HK's own convergence writes), and it inverts the source-of-truth direction.

Both risk exactly the drift the boundary invariant forbids. Treat L3 as a spike with a written go/no-go, not a committed feature. Likely outcome: keep writes in dotagents; if HK-authored edits are wanted, add a `dotagents import` path that pulls a specific HK change into the canonical store deliberately, rather than a live bridge.

## Codebase seams (verified in-tree)

- `cmd/dotagents/deps.go` — `deps check`/`update`, already wraps external-package-age. Home for L1.
- `cmd/dotagents/setup.go`, `setup_scaffold.go` — first-run; add the opt-in HK prompt here.
- `cmd/dotagents/external.go`, `external_cli.go`, `cli_launch_test.go` — external tool materialization + launch. Model for L2.
- `cmd/dotagents/detect.go`, `harness.go` — harness detection; useful if HK install is only offered when ≥1 supported harness is present.
- `cmd/dotagents/report.go`, `inspect.go` — the `status`/`inspect` output; where a "view in HarnessKit" pointer could surface.

## Open questions (need verification, do not guess)

1. **HK install method** — GitHub release binary vs `cargo install` vs `brew`. Check the repo's releases/install docs before wiring L1.
2. **HK headless/CLI mode** — does HK expose a non-interactive "start server on port X, no onboarding" invocation suitable for `dotagents view`? The web UI ran a 3-step onboarding wizard on first open; confirm it can be skipped/scripted.
3. **Config-root targeting** — can HK be pointed at an arbitrary config root (for `--config`/`$DOTAGENTS_HOME` setups), or does it only scan default `~/.<agent>` paths? Observed it reading default paths (`~/.omp`, `~/.hermes`); custom-root support unconfirmed.
4. **Pluggable write backend (L3)** — does HK have any hook/API to delegate mutations? Assume no until shown.
5. **Publish-age policy fit** — HK releases cadence vs the 3-day external-package rule; pick a window.

## Recommended first slice

Ship **L0 + L2** first; hold L1's auto-install behind the package-age answer:

1. L0 docs pointer (README + comparison + skill).
2. `dotagents view` that launches HK against the resolved config root, read-only framing. Answer open questions #2 and #3 as part of this slice.
3. Then L1 opt-in install once #1 and #5 are settled.
4. L3: separate spike doc, no code, decision recorded here.
