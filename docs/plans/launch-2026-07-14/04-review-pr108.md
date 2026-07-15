# Review — PR #108 "Prepare dotagents public launch" (head 7f9be732)

Reviewer subagent, 2026-07-14 ~19:50 +04. Diff: origin/main...origin/launch-prep.
Spec: 01-builder-spec.md §1–§7, 00-launch-plan.md.

## BLOCKERS

None for the documented "clone this repo → dotagents setup" path. Destructive
operations are ownership-scoped; materialization is atomic with rollback.

## LAUNCH RISKS

1. **Pi MCP dropped vs spec §4/§6.** `agentPi` has no MCP target and
   `dotagents.yaml` omits pi from every `mcp_servers[].agents`. README/landing
   honestly show Pi MCP `--` (no false claim), but the spec asked for MCP
   parity. Needs explicit user sign-off (ship skills-only for Pi) or wiring.
2. **First sync/setup is network-coupled.** `syncExternalRepos` always clones
   mattpocock/skills when the cache is absent and re-materializes over the
   already-committed copies; if the clone or pinned-SHA checkout fails
   (offline, upstream gone), the entire sync aborts and no harness is
   reconciled. Fix: skip re-materialization when committed copies match the
   lock, or degrade the failure to a warning.
3. **`sync` hard-aborts on a README without generated markers**
   (`sync.go:78` / `agents.go:308-345`). Fine for this repo, breaks the
   "bring your own ~/.agents repo" story. Fix: treat missing markers as no-op.
4. **Landscape table numbers unverified** (rulesync "40+", ruler "35+", feature
   cells) — must be human/web-verified before publishing.
5. **`doctor` audit over-scans `skill_dirs` with a `skills` allowlist** —
   latent false positive, not triggered by shipped config.

## NOTES

- Sourcery CI "failure" = review-posting noise: one typo nitpick + one
  opengrep false positive on `detectVanillaPi`'s `exec.Command` (input from
  YAML detect string via LookPath, not injectable). No action.
- Codex zero-copy marketplace deviates from literal `./` (uses `./skills` +
  `skills/.codex-plugin/plugin.json`), validated by a live remote install
  ("21 plugin skill commands, 0 symlinks") + `codex_layout_test.go`.
- `pull` alias prints a rename notice each run → cron mail nag. Cosmetic.
- Landing demo terminal lists `pi` but no `omp` line. Cosmetic.
- Rendered artifacts are idempotent; the ff-only cron won't break.

## Spec compliance

§1 CLI: implemented (4 verbs, groups, hidden aliases with notices).
§2 delivery exclusivity + doctor (dual-delivery, orphan cache): implemented.
§3 root cleanup: implemented (marketplace source deviation, validated).
§4 pi/omp split: partial — split/detect/roles correct, **Pi MCP missing**.
§5 external unification: implemented (materialize + lock ownership +
collision guard; grill-me and grilling both kept).
§6 README pass: implemented; count generated (20, grill-me excluded);
landscape numbers pending verification.

## Merge recommendation (reviewer)

Mergeable for the clone-this-repo path. Before broad publication: fix LR-2
and LR-3 (they undercut the BYO-repo story), get sign-off on LR-1 (Pi MCP
scope), verify LR-4 (landscape numbers). Sourcery is noise.
