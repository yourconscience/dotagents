# HarnessKit integration — design notes

Status: L0 + L2 shipped (`dotagents view`). L1 (opt-in install) and L3 (write-through) remain future work. Original design date 2026-09-06.

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

Install method is still open (release binary vs `cargo install` vs `brew` tap) — do not hardcode one until verified.

### L2 — Launch command (`dotagents view`)

`dotagents view` starts `hk serve`, prints the tokenized URL on its own line, and (locally) opens it in the default browser; `--no-open` skips the launch and `--ssh-host user@host` prints an `ssh -L` tunnel command instead.

- HarnessKit does its own harness discovery over the native homes (`~/.claude`, `~/.omp`, `~/.hermes`, …), so `view` does not load or pass the dotagents config root; a nonstandard `--config`/`$DOTAGENTS_HOME` only relocates dotagents' YAML, not the harness homes HK reads.
- Spawns the HK local server (127.0.0.1, token in URL) and opens it locally (suppressible with `--no-open`). Mirrors the external-CLI launch path (`external_cli.go`, `cli_launch_test.go`).
- Inspection intent, not enforced: `hk serve` has no read-only mode, so HarnessKit's own enable/disable/deploy actions can still write native dirs and bypass dotagents. The launch banner warns against using them on managed surfaces; reconcile drift with `dotagents sync`.

L0–L2 are the concrete near-term scope. All three keep the boundary invariant trivially (no managed-surface writes).

## Recommended first slice

**L0 + L2, now unblocked** (open questions #1–#3 resolved in HK's favor):

1. L0 docs pointer — README + `dotagents` SKILL + CLI help. Pointer only, no duplicated harness-compat table.
2. `dotagents view` — thin launcher: `exec.LookPath("hk")`, forward args to `hk serve`, inspection framing (writes not enforced — banner cautions), install hint when absent. Implemented on this branch (`cmd/dotagents/view.go`, `view_test.go`).
3. L1 opt-in install — deferred until the install method and publish-age window are settled.
4. L3 write-through — separate research spike, no code; keep dotagents the only writer until a go/no-go is decided.
