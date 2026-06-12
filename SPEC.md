# SPEC: dotagents public release

Executor: `codex exec` (GPT-5.5-high), one milestone per run, in order. Each milestone has machine-checkable acceptance criteria; do not start the next milestone until the current one passes. Human gates are marked HUMAN GATE and must stop the run with a clear summary for the user.

## Goal

Make this repo public and presentable: a stranger can (a) install the portable skills into Claude Code as a plugin, (b) install the `dotagents` CLI and run their own canonical `~/.agents` repo using this one as the living reference, and (c) read a single GitHub Pages landing page that shows the project and three happy-path scenarios.

Positioning: tool + living example. The landing sells the `dotagents` CLI and the canonical-repo pattern; this repo is the reference implementation, not something strangers sync verbatim.

## Non-goals

- No multi-page docs site, no SSG framework. One static page.
- No new skills or CLI features. Release what exists.
- No rewrite of README; the landing complements it.
- No template repo; fork-and-edit of this repo is the documented path.

## Milestones

### M-pre: remove omx

omx (oh-my-codex) is retired from the machine and from repo surfaces.

Acceptance:
- `omx uninstall` has been run; `command -v omx` exits non-zero (manual removal of the binary is allowed after uninstall).
- `grep -rn --include="*.md" "omx" README.md skills/ plugins/dotagents/skills/` returns no hits (history/experimental notes may stay).
- `codex exec "echo ok"` works on the cleaned setup.

### M0: history audit, then flip to public

HUMAN GATE before the flip.

Steps:
1. Run gitleaks (or trufflehog) over the full history: `gitleaks detect --source . --log-opts="--all"`. Zero findings, or each finding explicitly waived by the user.
2. Produce `tmp/audit-history.md`: every file ever committed (`git log --all --name-only --format= | sort -u | grep -v '^$'`), flagging personal-data candidates (jobs pipeline, telegram/linkedin configs, memory hooks, names/emails beyond git identity).
3. HUMAN GATE: user reviews the audit summary and approves the flip.
4. After approval: `gh repo edit --visibility public`.

Acceptance:
- Secret scan clean (or waivers recorded in the audit file).
- `gh repo view --json visibility` returns `PUBLIC`.
- `tmp/audit-history.md` exists locally and is NOT committed (`tmp/` is gitignored).

### M1: verify all 8 harness integrations

Two-tier bar. Tier 1 (all 8: Claude Code, Codex, Droid, Hermes, Pi/OMP, OpenCode, OpenClaw, Amp): scripted, deterministic. Managed harnesses get sync/native-location/status assertions; compatibility-only harnesses get documented-limitation assertions. Tier 2 (harnesses with a headless mode: Claude Code, Codex, Droid, Hermes): a live skill invocation.

Steps:
1. Write `tmp/verify-harnesses.sh` (not committed): for managed harnesses, run `dotagents sync`, assert skills/MCP land in the harness's native location, and confirm `dotagents status` reports clean; for compatibility-only harnesses, assert the README documents the limitation instead of requiring a managed native surface.
2. Tier 2 live runs, one cheap prompt each, asserting the agent can see a synced skill:
   - `claude -p "list available skills, one per line"` contains `tech-search`
   - `codex exec "list available skills"` contains `tech-search`
   - `droid exec` equivalent
   - `hermes -p` equivalent
3. Claude Code plugin path verified separately from sync: `claude plugin` marketplace add/install flow works against the public repo (after M0), or against the local path pre-flip.
4. Record results in `tmp/verify-results.md` with exact commands and pass/fail per harness.

Acceptance:
- `tmp/verify-harnesses.sh` exits 0 covering all 8 harnesses.
- 4 live-run transcripts captured in `tmp/verify-results.md`, each showing a synced skill visible to the agent. A transcript captured by an authenticated external session (e.g. the user's Claude Code) counts; the executor must not re-run checks that require auth its environment does not have — treat the recorded transcript as the evidence.
- `dotagents status` clean globally.
- Any harness that cannot pass gets a fix or a documented limitation in README (compatibility table), not silence.

### M2: landing page

One static page, hand-written HTML/CSS in `docs/site/index.html` (style may echo `docs/harness-map.html`). No build step, no framework.

Content:
- What dotagents is (tool + canonical-repo pattern, this repo as living reference).
- Three happy-path scenarios with exact commands:
  1. Claude Code plugin: marketplace add + install.
  2. Your own `~/.agents`: fork this repo (or start clean), install CLI, `dotagents setup`, `dotagents sync`.
  3. Add a skill once, it appears in every harness: create `skills/foo/SKILL.md`, `dotagents sync`, show it visible in 2+ harnesses.
- Comparison table reused from README (dotagents vs skillshare/vsync/agents-cli).
- Link to repo and releases.
- Footer easter egg: one small muted line linking to https://yourconscience.github.io/mushroom/ with a 🍄 icon and a short caption (e.g. "agent-built in one evening"). Keep it footer-only; do not feature it in the main content.

Copy process: draft -> humanizer pass (skill is synced and available) -> HUMAN GATE: user reviews and approves the final copy before deploy. Do not deploy unreviewed copy.

Acceptance:
- `docs/site/index.html` exists, self-contained (no external JS deps; system fonts or one hosted font max).
- Every command shown on the page has been executed verbatim during M1/M2 and worked.
- User approval recorded (gate passed) before M3.

### M3: deploy and release

Steps:
1. GitHub Pages via Actions workflow deploying `docs/site/` on push to main.
2. Tag a release `vX.Y.Z` (next semver from existing tags); goreleaser workflow already attaches binaries.
3. README: add the Pages URL at the top.

Acceptance:
- Pages URL returns 200 and renders the landing.
- Release exists with macOS/Linux amd64+arm64 binaries attached.
- `go install github.com/yourconscience/dotagents/cmd/dotagents@latest` works from a clean environment (now that the repo is public).

## Constraints

- All repo changes on feature branches, PRs into main; `/pr-triage` discipline applies. No worktrees.
- Commit messages: short single line, no co-author trailers.
- Do not commit `tmp/` artifacts, audit outputs, or verification transcripts.
- Quota awareness: heavy execution on Codex (OpenAI Pro); Claude only for humanizer pass and reviews.
- Anything irreversible (visibility flip, tag push, Pages enable) happens only at its HUMAN GATE or explicitly listed acceptance step.

## Risks / open questions

- History audit may surface personal data that requires history rewrite before flip; if so, stop and present options (filter-repo vs clean mirror) instead of proceeding.
- OpenClaw/Amp/OpenCode local installs may be stale or broken; budget for "documented limitation" outcome rather than deep debugging.
- Plugin marketplace flow against a just-flipped repo may need GitHub cache time; retry before declaring failure.

## Codebase notes

- CLI: `cmd/dotagents`; release pipeline: `.github/workflows/release.yml` (goreleaser on `v*` tags).
- CI: `.github/workflows/ci.yml` (build-and-lint matrix + skill-frontmatter).
- Existing static-page style reference: `docs/harness-map.html`.
- Skills inventory and categories: README "Skills" section, `docs/reports/skill-categorization.md`.
- Plugin marketplace manifest: `.claude-plugin/marketplace.json` (portable skills list).

## Outcome / Deviations

(fill after implementation)
