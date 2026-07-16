# dotagents launch plan — 2026-07-14

> Superseded on 2026-07-16 by `06-separation-design.md` and the root `SPEC.md`.
> The current user request explicitly authorizes merging the separation PR
> after checks pass and all active review comments are resolved.

> **MERGE POLICY (hard constraint, added 2026-07-14 13:20 +04 by the user via Claude):**
> Do NOT merge anything into `main` and do NOT push to `main`. Any prior instruction to
> merge everything is revoked for `main`. Your endpoint: push your work as a branch, open
> a PR, post the self-verification outputs, and STOP. Review and the merge into `main`
> will be done separately by Claude on the M1 machine after human review.


Master plan for the public launch push. All decisions below were confirmed in the
grilling session on 2026-07-14; specs for execution live next to this file.

## Sequencing

| When | What | Who |
|---|---|---|
| Today (2026-07-14) | Plan + specs + post drafts written | Claude (this session) |
| Today | Implementation per `01-builder-spec.md` in the `launch-prep` worktree | gpt-5.6-sol (builder) |
| Today | Builder self-verifies: unit tests + local run of setup/status/sync/doctor | gpt-5.6-sol |
| Today/tonight | End-to-end validation per `02-tester-spec.md` (same env or clean machine) | tester agent |
| Tomorrow (2026-07-15) | User reviews diff + posts, then publishes LinkedIn + X | user |
| ~2026-07-17/18 | Show HN, after first external feedback is folded in | user |

Nothing is published until the CLI simplification, Pi/OMP split, and README pass
have landed and a clean install has been verified. The landing page people hit
from the post and the post itself are one artifact.

## Confirmed decisions (log)

1. **CLI surface**: public verbs are `setup / status / sync / doctor`; `skill`
   and `mcp` are second-tier subcommand groups; `cron`, `memsearch`,
   `deps update` are hidden (`help --all`). Nothing deleted yet — deletions are
   cheaper after launch. Full mapping in `01-builder-spec.md`.
2. **Repo root**: remove the legacy rendered Codex plugin copy. The completed
   zero-copy layout packages the canonical `skills/` tree directly.
   Remove `SPEC.md` and `fable.md`. Keep `SOUL.md` at root. Keep `.mailmap`
   and tooling dotfiles.
3. **Delivery exclusivity**: plugin and sync delivery must be mutually
   exclusive per harness; `doctor` detects violations and orphaned plugin
   caches; `setup --delivery` switches cleanly.
4. **Pi/OMP**: two separate harness entries (`pi`, `omp`), no hybrid anywhere.
   Pi is first-party in the README table; OMP is a footnote with an asterisk.
   Roles render for OMP only — vanilla pi has no subagents, by design.
5. **grill-me / grilling**: keep both — `grill-me` is an upstream-designed
   alias command for the `grilling` skill. README must not count the alias;
   the skill count becomes generated.
6. **Posts**: main hook everywhere is **"Dotfiles for your AI agents"**.
   Secondary angle: supply-chain pinning (`dotagents.lock` + audit) as the
   differentiator. Tertiary: the unification insight — every harness converges
   on the same concepts (skills, MCP, hooks, roles) with different spellings,
   so one canonical layer can render to all. Framed as a generalization, not a
   fragmentation-zoo table.
7. **Channels**: LinkedIn + X first (EN, personal account, same substance),
   HN 2–3 days later. X is a short thread; LinkedIn is a single narrative
   post. Drafts go through `x-sim` before publishing.
8. **Stars**: no growth hacks. Distribution = posts + tagging upstream authors
   (Matt Pocock — his skills are pinned as externals; badlogic — Pi support) +
   2–3 communities + awesome-list PRs. First ~20 stars from the personal
   network in day one is normal, not gaming. Claude coordinates all community
   posting (drafts + calendar); user only copy-pastes and replies.

## Files

- `01-builder-spec.md` — implementation spec for gpt-5.6-sol
- `02-tester-spec.md` — end-to-end validation matrix
- `03-posts.md` — LinkedIn / X / HN drafts + community calendar

## Known local facts feeding the specs

- `~/.claude/plugins/cache/yourconscience/` holds an orphaned dotagents plugin
  cache (absent from `installed_plugins.json`) — doctor must catch this class.
- Before the split, the `pi` target incorrectly pointed at OMP's skill root.
  The completed config now keeps Pi under `~/.pi/agent/` and OMP under
  `~/.omp/agent/`, with capabilities verified independently.
- The user reports duplicate skill listings (bare name + `dotagents:` prefix)
  possibly only on the M4 machine — the dual-delivery guard addresses the
  class; tester reproduces.
- A cron auto-pull test is live on the main machine today — the builder must
  not change cron behavior, only hide the command from short help.
- `skills/` currently has 19 entries; README claims 18 and lists `grill-me`
  but not `grilling`.
- External skills arrive via two different mechanisms today: materialized
  copies in `skills/` (grill-me, grilling) vs `external/` + direct symlinks
  (wayfinder, domain-modeling). Unify (see builder spec §4).
