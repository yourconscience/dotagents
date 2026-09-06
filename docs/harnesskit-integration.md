# HarnessKit integration — design notes

Status: draft / thinking. Branch: `feat/harnesskit-integration`. Date: 2026-09-06.

## Finding

[HarnessKit](https://github.com/RealZST/HarnessKit) (RealZST/HarnessKit, Rust, Apache-2.0, ~420★, active) is a web UI (also desktop/CLI) that inspects and manages agent extensions, configs, memory, and rules across 13 harnesses. Verified live against a full stack install on 2026-09-06: it detects and reads the **full dotagents stack** — Claude Code, Codex, **Oh My Pi** (`~/.omp/agent/`), **Hermes** (`~/.hermes/`), plus Gemini CLI, Copilot, OpenCode, Grok Build. This is exactly the coverage (Pi/OMP + Hermes) that CCO and ai-config-sync-manager lack.

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

## Open questions

Resolved 2026-09-06 by inspecting `hk` 1.10.0 (`hk --help`, `hk serve --help`, `hk list --help`):

1. **HK headless/CLI mode — RESOLVED (yes).** `hk serve` is fully scriptable: `--port`, `--host`, `--token`/`--no-token`, `--name`. The 3-step onboarding is client-side UI state, not a server gate. HK also ships a pure CLI — `hk status`, `hk list --json`, `hk audit`, `hk info`, `hk enable/disable` — so a future text/status integration can consume `hk list --json` without the web server.
2. **Config-root targeting — RESOLVED (no flag, not needed).** `hk serve`/`hk list` have no `--config`/`--root`; only `HK_SCOPE_LAST_USED` (scope memory). HK reads the native harness homes (`~/.claude`, `~/.codex`, `~/.omp`, `~/.hermes`), which is exactly what dotagents materializes — so `view` targets the right thing for the default `~/.agents` root. Custom `$DOTAGENTS_HOME`/`--config` only relocates dotagents' YAML, not the harness homes HK reads, so no retargeting is required.
3. **HK binary/version — RESOLVED.** `hk` 1.10.0, Mach-O arm64, installed at `~/.local/bin/hk`.

Still open (gate L1, not L2):

4. **HK install method (L1)** — release binary vs `cargo install` vs `brew` tap. Verify from HK's releases/install docs before wiring an opt-in installer.
5. **Publish-age policy fit (L1)** — HK's release cadence vs the 3-day external-package rule (`package_age.go`); pick a window.
6. **Pluggable write backend (L3)** — does HK expose any hook/API to delegate mutations? Assume no until shown.

## Recommended first slice

**L0 + L2, now unblocked** (open questions #1–#3 resolved in HK's favor):

1. L0 docs pointer — README + `dotagents` SKILL + CLI help. Pointer only, no duplicated harness-compat table.
2. `dotagents view` — thin launcher: `exec.LookPath("hk")`, forward args to `hk serve`, read-only framing, install hint when absent. Implemented on this branch (`cmd/dotagents/view.go`, `view_test.go`).
3. L1 opt-in install — deferred until #4 and #5 are settled.
4. L3 — separate spike, no code; decision recorded here.
