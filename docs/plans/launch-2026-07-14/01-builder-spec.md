# Builder spec — dotagents launch prep

> **MERGE POLICY (hard constraint, added 2026-07-14 13:20 +04 by the user via Claude):**
> Do NOT merge anything into `main` and do NOT push to `main`. Any prior instruction to
> merge everything is revoked for `main`. Your endpoint: push your work as a branch, open
> a PR, post the self-verification outputs, and STOP. Review and the merge into `main`
> will be done separately by Claude on the M1 machine after human review.


Target: `launch-prep` worktree (`.worktrees/launch-prep`). Builder: gpt-5.6-sol.
Work style per repo AGENTS.md: surgical changes, simplicity first, tests with
each behavior change, verify before reporting. Anything marked **RESEARCH**
must be confirmed against live docs/registries, not memory.

## 1. CLI simplification

Public surface becomes exactly four verbs + two subcommand groups. Default
`--help` shows only these; everything else appears in `dotagents help --all`.

### Command mapping (from → to)

| Current | New | Notes |
|---|---|---|
| `setup` | `setup` | absorbs delivery choice, see §2 guard |
| `status` | `status` | absorbs `external list` (lock state section) and `memsearch status` (one line) |
| `sync` | `sync` | absorbs `render` (agent/plugin artifacts always regenerated inside sync); `pull` becomes `sync --pull` |
| `doctor` | `doctor` | absorbs `deps check` (package age), `sources` (external source availability), `dogfood` → `doctor --e2e`; new checks in §2 |
| `pull` | `sync --pull` | keep old name as hidden alias for the live cron entry; do NOT break the user's installed crontab line |
| `render` | (gone) | folded into `sync`; adapt the codex-plugin drift doctor check to whatever remains after §3 |
| `plugin add/remove` | `setup --delivery plugin\|sync` | re-running setup switches delivery |
| `dogfood` | `doctor --e2e` | |
| `deps check` | inside `doctor` | |
| `deps update` | hidden | unchanged behavior |
| `sources` | inside `doctor` | |
| `skillify <name>` | `skill new <name>` | |
| `external update [name]` | `skill update [name]` | visible subcommand — regular user operation |
| `promote <name>` | `skill promote <name>` | second tier |
| `mcp list/add/import/remove` | `mcp …` | unchanged, second tier; README points at editing `dotagents.yaml` as the primary path |
| `cron …` | hidden | behavior unchanged — a live cron test runs on the main machine today |
| `memsearch …` | hidden | candidate for extraction post-launch, not now |

Help text: top-level help lists `setup status sync doctor` with one-line
descriptions, then a single line: `skill`, `mcp` groups; footer: `dotagents
help --all` for the rest. Keep old command names working as hidden aliases for
one release where cheap (pull, external update), so nothing scripted breaks.

## 2. Delivery exclusivity guard

Problem observed in the wild: dotagents installed both via sync symlinks and
as a Claude Code plugin → every skill listed twice (`name` and
`dotagents:name`).

- `setup --delivery plugin`: installs plugin delivery AND removes managed sync
  symlinks for that harness. `--delivery sync` does the inverse (uninstalls or
  disables the plugin where the harness supports it; at minimum instructs and
  verifies).
- `sync` refuses (with a clear message) to symlink into a harness whose
  delivery is set to plugin.
- `doctor` gains checks:
  - **dual-delivery**: managed symlinks present AND dotagents plugin
    installed/enabled for the same harness → error with the one-command fix.
  - **orphaned plugin cache**: e.g. `~/.claude/plugins/cache/yourconscience/`
    present but plugin absent from `installed_plugins.json` → warn, offer
    cleanup instruction.
- State: delivery mode is recorded per harness in `dotagents.yaml`
  (existing `delivery:` field on the agent entry is the source of truth).

## 3. Repo root cleanup

- **Delete the legacy rendered Codex plugin copy.**
  - `.agents/plugins/marketplace.json` points the `dotagents` package at
    `./skills`.
  - `skills/.codex-plugin/plugin.json` declares `"skills": "./"`.
  - Codex therefore packages the canonical skill tree directly, without a
    committed mirror or a regeneration step.
- Remove `SPEC.md` (delete file; move nothing). Keep `SOUL.md` at root.
- `fable.md` is untracked scratch on the main checkout — do not add; note for
  the user to delete.
- Keep `.mailmap`, `.agnix.toml`, `.golangci.yml`, `.goreleaser.yaml`,
  `go.work` as-is.
- Resulting visible root: `agents/ cmd/ docs/ experimental/ hooks/ mcp/
  memory/ scripts/ skills/` + `README.md AGENTS.md CLAUDE.md LICENSE SOUL.md
  dotagents.yaml dotagents.lock go.work`.

## 4. Pi/OMP split

Kill the hybrid. Two independent harness entries:

- `pi` — upstream vanilla Pi. Detect via `pi`; sync skills to
  `~/.pi/agent/skills`. Pi has no native MCP, hook, or subagent-role surface,
  so dotagents must not invent one.
- `omp` — oh-my-pi. Detect via `omp`; sync skills to
  `~/.omp/agent/skills`, MCP to `~/.omp/agent/mcp.json`, and native roles to
  `~/.omp/agent/agents`.
- Both installed means both are detected and synced independently.
- Note: `~/.omp/agent/skills` does not exist on the user's machine today
  despite the current config claiming to sync there — find out why (path
  wrong? sync skipped? OMP reads another dir?) and fix; tester will verify.

## 5. External skills mechanism unification

Two mechanisms exist today: materialized copies in `skills/` (grill-me,
grilling from mattpocock/skills) vs `external/` checkout + direct symlinks
into harness skill dirs (wayfinder, domain-modeling). Unify on **materialized
copies in `skills/`** (plays well with plugin installs and Codex's symlink
allergy), driven by `dotagents.lock` pins; `skill update` refreshes them.
`doctor` flags any external skill delivered outside the unified mechanism.

Keep both `grill-me` (alias command, `disable-model-invocation: true`) and
`grilling` (the actual skill) — upstream designs them as a pair.

## 6. README pass

- Harness table: separate `Pi` (skills yes; roles, MCP, hooks no) from `OMP`
  (skills, roles, and MCP yes; hooks no). Do not describe the two binaries or
  config roots as one harness.
- Key commands block → the four verbs plus one line for `skill` / `mcp`
  groups. No `render`, no `plugin add`, no `deps check`.
- Skill count: generated (a `sync`-rendered README fragment or a doctor check
  that fails on drift — builder's choice, simplest wins). Alias `grill-me`
  not counted.
- Landscape table: **RESEARCH** — re-verify rulesync / ruler / openskills
  current facts (tool counts, features) from their repos as of 2026-07-14 and
  update numbers; keep the framing (project-level vs user-level).
- Regenerate `docs/harness-map.png` for the Pi/OMP split (also used in
  launch posts).
- Install section unchanged except any path fallout from §3.

## 7. Self-verification (before handing to tester)

- `go test ./...` green in the worktree.
- Run `setup`, `status`, `sync`, `doctor`, `doctor --e2e` locally against the
  worktree copy; paste outputs into the PR description.
- `dotagents --help` output matches §1 exactly.
- Do not touch the live crontab, the user's `~/.claude/skills` outside
  guard-relevant behavior you can revert, or the running cron test.
