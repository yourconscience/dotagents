# Tester spec — dotagents launch prep e2e

Runs after the builder lands `01-builder-spec.md` and self-verifies. Tester is
report-only: record findings, do not fix. Environment: the user's machine
(current env) for the harness-integration matrix, plus a clean machine/VM (or
pristine user account) for install-path tests. Anything requiring sudo or a
new VM: ask the user.

## A. Delivery matrix (Claude Code — the critical one)

For each state, verify skill listing in a fresh Claude Code session shows each
skill exactly once, with the expected name form:

| State | Expected |
|---|---|
| sync-only (default) | bare skill names, no `dotagents:` prefixes |
| plugin-only (`setup --delivery plugin`) | `dotagents:`-prefixed skills, no bare-name symlinks in `~/.claude/skills` |
| both simultaneously (force the broken state by hand) | `doctor` flags dual-delivery with the exact fix command; after the fix, back to a clean single listing |
| orphaned plugin cache (cache dir present, plugin not installed) | `doctor` warns; reproduce from the real artifact at `~/.claude/plugins/cache/yourconscience/` |

Known repro hint: the duplicate listing was observed possibly only on the M4
machine — if it doesn't reproduce on the main machine, run the matrix there too.

Behavioral parity: pick 3 skills (one simple, one with tools/scripts, one
external e.g. `grilling`) and invoke each under sync-only and plugin-only;
results must be equivalent.

## B. Codex plugin path

- Install the Codex plugin from the marketplace using the `skills/` package
  root on a machine/account without dotagents sync. Skills must load from the
  canonical tree and no rendered duplicate should exist.
- Same dual-delivery check as A if Codex supports both paths.

## C. Pi and OMP

- Install upstream `pi` (this is the task where pi gets installed on the
  user's machine — coordinate before touching global state).
- `dotagents status` detects both `pi` and `omp`; `sync` delivers skills to
  both roots. Verify `~/.omp/agent/skills` actually exists and OMP loads
  skills from it (the dir was absent before the fix — confirm root cause was
  addressed).
- Roles: rendered for OMP in its native format and loadable; NOT rendered for
  pi.
- MCP config lands only for OMP at `~/.omp/agent/mcp.json`; vanilla Pi has no
  native MCP surface and must receive no MCP file.

## D. Clean-machine install paths

On a pristine account/VM, run each README install path top to bottom exactly
as written:

1. curl installer → `git clone … ~/.agents` → `dotagents setup`
2. `/plugin marketplace add yourconscience/dotagents` + `/plugin install`
   (plugin-only, no CLI)
3. `go install …@latest`

Record every prompt, error, or README mismatch verbatim.

## E. CLI surface

- `dotagents --help` shows exactly `setup status sync doctor` + `skill`/`mcp`
  groups; `help --all` reveals hidden commands.
- Hidden aliases still work: `pull` (the crontab on the main machine uses it —
  verify today's scheduled run succeeded and synced), `external update`.
- `doctor --e2e` passes on a healthy setup and fails informatively on a broken
  one (e.g. remove a symlink by hand).

## F. README truth check

Every number and claim in the rendered README verified against the repo:
skill count (alias excluded), harness table rows, landscape table facts,
commands block matches actual `--help`.

## Reporting

One markdown report: finding per row — area, repro steps, expected vs actual,
severity (blocker / launch-risk / cosmetic). Blockers = anything that breaks
a README install path or produces duplicate/missing skills.
