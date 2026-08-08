# anywhere-agents feature-borrowing report for dotagents

Scope: `/Users/conscience/Public/anywhere-agents` (maintainer: Yue Zhao, USC/PyOD) vs. `/Users/conscience/Workspace/dotagents` (this repo, branch `plugin`). Read at code level on both sides: `guard.py`, `statusline.py`, `agent-quota.py`, `generate_agent_configs.py`, `AGENTS.md`, `bootstrap.sh`, `packs.yaml`, all four `SKILL.md` files, `.claude/settings.json`, `user/settings.json` on the anywhere-agents side; `hooks.go`, `harness.go`, `inspect.go`, `sync.go`, `context_cost.go`, `session_hub.go`, `review.go`, `starter_assets.go`, `root_instructions_test.go`, `AGENTS.md`, `README.md`, `skills/dotagents/SKILL.md` on the dotagents side.

Note on naming: anywhere-agents' README lists `iannuttall/dotagents` under "Different approaches" as a related project. That is a different, unrelated `dotagents` repo, not `yourconscience/dotagents` (this one). No relationship between the two exists beyond the name.

**Filing note**: this file is a personal strategy memo, not public distribution content. dotagents' own `AGENTS.md` states the repo "must not contain a maintainer's personal skills, hooks, MCP servers, secrets, memory data, or machine-specific configuration." Leave this file untracked; do not commit it.

---

## 1. Feature inventory

anywhere-agents is a single-maintainer, single-user, Claude Code + Codex opinionated configuration, distributed as a bootstrap script plus a pack CLI (PyPI/npm). It is explicitly not a general-purpose sync tool (its own README says so). Notable features:

- **Bootstrap script** (`bootstrap.sh`/`.ps1`, 455 lines): fetches shared `AGENTS.md`, skills, `.claude/settings.json`, and user-level hooks/statusline from a Git remote via sparse clone, on every session start (via a SessionStart hook) or on demand.
- **`AGENTS.md` as tagged single source**: one file with `<!-- agent:claude -->` / `<!-- agent:codex -->` HTML-comment blocks; `generate_agent_configs.py` extracts per-agent content into `CLAUDE.md` and `agents/codex.md`. Untagged content is shared.
- **`guard.py` PreToolUse hook** (approximately 1500 lines): five separate gates in one file: (0) auto-allow for three trusted shipped PowerShell scripts, (1) writing-style deny (banned AI-tell words on prose file writes), (2) session-banner deny (blocks nearly all tool calls until the model emits a banner and writes an ack file), (3) compound-`cd` deny, (4) mandatory-ask classifier for destructive git/gh, package publish, and file/device deletion, with wrapper-piercing (ssh, bash -c, docker exec, pwsh -Command) up to a depth limit.
- **`statusline.py`**: Claude Code `statusLine` command. Reads `rate_limits` from Claude's own statusLine stdin payload (native field, Claude Code v2.1.80+, Pro/Max only) and tails the newest Codex `~/.codex/sessions/**/rollout-*.jsonl` for Codex's `payload.rate_limits`. Renders one line: `🤖 Opus 4.7 · 5h 78% (3h4m) · 7d 51% (15h4m)  |  Codex 5h 89% (3h25m) · 7d 90% (4d23h)`. Also persists Claude's rate_limits to `~/.claude/rate-limits-cache.json` as a side effect.
- **`agent-quota.py`**: a second, standalone CLI that reads the same two data sources (the persisted Claude cache plus the Codex rollout tail) so quota is visible from a plain shell or a Codex session, not just inside a Claude Code statusline render.
- **`session_bootstrap.py`**: SessionStart hook that writes `session-event.json`, which `guard.py`'s banner gate reads to decide whether to block tool calls until a banner is emitted.
- **Pack system** (`scripts/packs/`, `compose_packs.py`, `bootstrap/packs.yaml`): a v2 manifest format for "passive" packs (file drops into `AGENTS.md`) and "active" packs (skills/hooks/commands), with a 4-method Git auth chain, drift detection against a lock file, and a `pack add/remove/list/verify` CLI writing to a user-level XDG config.
- **Four shipped skills**: `implement-review` (dual-agent review loop, Claude implements/Codex reviews, with a Phase-0 plan-review step), `my-router` (keyword/file-type dispatcher to the other skills), `ci-mockup-figure` (HTML/TikZ figure generation for papers), `readme-polish` (README rewrite to 2025-2026 GitHub conventions).
- **Session-start banner**: a fixed-format status line (OS, Claude/Codex versions plus drift, effort level, active skills by tier, hook presence, session-check result) that the model is required to emit as the literal first text of specific session turns, enforced by the banner gate in `guard.py`.
- **Fork-and-replace distribution model**: no plugin/marketplace; the README's explicit customization path is "fork the repo, edit `AGENTS.md`, point your bootstrap at your fork."

## 2. Borrowable features

### 2a. Writing-style guard ("humanizer")

**What it does in anywhere-agents**: `check_writing_style` in `guard.py` denies `Write`/`Edit`/`MultiEdit` on `.md`/`.tex`/`.rst`/`.txt` files if the outgoing content matches any of approximately 45 banned AI-tell words (with inflection variants and code-fence/inline-code stripping to avoid false positives on meta-discussion). The deny message includes a concrete `Suggested rewrite:` per hit. Two escape hatches: `AGENT_STYLE_HOOK=off` (this gate only) and the legacy `AGENT_CONFIG_GATES=off` (this gate plus the banner gate).

**Charter reality check**: this is not a charter violation waiting to happen. dotagents already ships executable hook payloads as public starter content: `memory/hooks/*.py`, `memory/lib/*.py`, and `AGENTS.md` explicitly lists "reusable memory infrastructure under `memory/`" as intentional public distribution. A style-guard hook script is the same category of thing, not a new kind of thing.

**How it could work in dotagents**: dotagents' `hookConfig` (`cmd/dotagents/hooks.go`) already models `Event`, `Command`, `Timeout`, `Agents` and dispatches per-harness via `hookTargetForHarness`. Claude Code's target writes into `~/.claude/settings.json` hooks (`inspectClaudeHookMap`/`upsertClaudeHookMap`) using a `PreToolUse` matcher shape identical to what `guard.py` needs (`.claude/settings.json` in anywhere-agents literally registers `PreToolUse` with a `Bash`/`PowerShell` matcher, the same mechanism). Hermes maps a fixed `hermesHookEvent` table; that table lists `pre_tool_call` as a recognized native event, but dotagents has never wired a deny-capable hook payload through it, so Hermes parity needs verification before it's claimed. Codex hooks go through `patchCodexHooksFeature`/nested JSON; whether Codex's current hook surface supports permission-decision-style deny also needs verification before shipping.

Scope the word list to the user's own `SOUL.md`, not anywhere-agents' 45-word list. `SOUL.md` already states: no emoji, no em dashes, no filler openings/closings, telegraph style. That is a narrow, mechanically checkable set. Importing anywhere-agents' list verbatim would deny writes containing `acquire`, `gauge`, `vast`, `articulate`, `nuance`, `facet`: all legitimate terms in ML-infra and technical prose. A narrow SOUL.md-derived guard (deny em-dash-as-punctuation, deny a short handful of stock openers/closers, deny emoji in prose files) is defensible; the 45-word academic-writing list is not a good fit for this user's technical writing and should not be copied wholesale.

One direct inconsistency to flag if anything from anywhere-agents is copied verbatim: `statusline.py` renders `🤖` and the session banner format uses `📦`. SOUL.md says no emoji. Do not copy those glyphs into anything built for this user.

**Effort**: small as a first cut (a Python script plus one `hookConfig` entry plus Claude Code wiring only); medium to reach parity across harnesses (need to verify Hermes and Codex can actually express PreToolUse-deny before claiming the capability matrix supports it; dotagents' own architecture rule is "a surface gets a yes only after its native behavior is verified end to end").

**Public or private**: private (`~/.agents`). This is a personal writing-voice guard tied to `SOUL.md`, which is explicitly a private file (`~/.agents/SOUL.md`, not checked into the public dotagents repo). It is exactly the kind of "maintainer's personal... hooks" the dotagents charter excludes from the public distribution. If dotagents ever ships a *generic, configurable* style-guard hook framework (word list supplied via `dotagents.yaml`, no baked-in list) as public infrastructure, that is a different, larger product decision; do not conflate "ship this hook script" with "ship a hook framework."

### 2b. Common rules system (tagged single-source AGENTS.md)

**What it does in anywhere-agents**: one `AGENTS.md` with `<!-- agent:claude -->`/`<!-- agent:codex -->` blocks; `generate_agent_configs.py` extracts per-agent content into `CLAUDE.md` and `agents/codex.md` on every bootstrap run, with a `GENERATED FILE` marker that protects hand-authored files from being silently overwritten (falls back to a loud warning plus a rename-to-`.local.md` instruction).

**This partially already exists in dotagents, verified in code, not just README**: `harness.go` defines `RootInstructionsCapability` (`Path` = symlink location, `Expected` = target under the canonical config root), and `inspectRootInstructions`/`applyAgentRootInstructionSync` (`inspect.go`, `sync.go`) symlink a single shared `AGENTS.md` into place, with `stateConflict` detection when a non-symlink file already exists at the target. `root_instructions_test.go` covers missing/synced/conflict states end to end. **But this capability is wired for exactly one harness: Factory Droid** (`grep -c "RootInstructions:"` on `harness.go` returns 1, at the Droid block). Claude Code, Codex, and Hermes have no `RootInstructionsCapability` registered at all; dotagents does not currently sync a shared root-instructions file to any of them at the user level.

Two real gaps, not one:
1. No per-agent content differentiation. dotagents' symlink approach is identical-content-everywhere; anywhere-agents' tagged-block generator lets Claude-only and Codex-only content coexist in one source file. dotagents has no equivalent to `<!-- agent:X -->` blocks.
2. No Claude Code or Codex root-instructions sync at all today, only Droid.

**How it could work in dotagents**: two separable pieces of work.
- (a) Extend `RootInstructionsCapability` (or a same-shape sibling) to Claude Code (`~/.claude/CLAUDE.md`, or increasingly `~/.claude/AGENTS.md` per Anthropic's native-AGENTS.md support; verify current native support before choosing the target filename) and Codex (`~/.codex/AGENTS.md`). Straight symlink, same mechanism already proven for Droid. This is the smaller, lower-risk piece and fits the existing architecture with no new concepts.
- (b) A tagged-block generator (`<!-- agent:claude -->`) that renders differentiated content per harness from one canonical `AGENTS.md`, the way anywhere-agents' `generate_agent_configs.py` does. This is a genuinely new capability (a template/render step, not a symlink), needs its own generated-file-protection logic (anywhere-agents' `GENERATED FILE` marker plus warn-and-preserve pattern is worth copying directly; it is a good, low-risk idempotency contract), and a decision about whether dotagents' `agents/*.md` role files are a better existing home for per-harness prompt differentiation than inventing a second templating mechanism.

**Effort**: (a) small: extends a capability that already has tests and two other implementations to point at (Codex, Claude). (b) medium: new render/diff logic plus tests.

**Public or private**: (a) belongs in public dotagents: it is infrastructure (a verified per-harness adapter), same class as the Droid RootInstructions capability already public. (b), the templating/tagged-block mechanism, is also legitimately public infrastructure if built generically; the *content* of any given `AGENTS.md` written with it stays private in `~/.agents`.

### 2c. Statusline / quota display

**What it does in anywhere-agents**: `statusline.py` is wired as Claude Code's native `statusLine` command (`user/settings.json`: `"statusLine": {"type": "command", "command": ...}`). It reads Claude's own injected `rate_limits` from statusLine stdin (native Claude Code feature, no polling, no API key) and separately tails Codex's newest rollout JSONL for its `rate_limits` field. `agent-quota.py` is a second, non-statusline CLI reading the same two on-disk sources so quota is visible from a plain shell.

**This is a genuinely new surface for dotagents**, not an extension of an existing one: verified, `grep -ri "statusline"` across the entire dotagents Go tree and skill/doc content returns zero hits. The user's `USER.md` profile also explicitly says they want a "provider-agnostic quota tracking skill/command like `/quota`... for Claude, Codex, and other operators" and asks for approval before automation; this is a match to an already-stated want, not a speculative feature.

Precedent for adding a new, non-four-surface capability to dotagents already exists in this repo's own in-flight work: `session_hub.go` (untracked at the time of this analysis) registers `harness.Sessions{Store, Resume}` in the harness registry, a fifth capability that is not skills/MCP/hooks/roles. That means "add a small new managed capability outside the four charter surfaces" is already happening here, under the same inventory-procedure rule the charter states ("New starter content requires an explicit product decision and an inventory test update"), not something this report is proposing for the first time.

There is also a design precedent already in the codebase for exactly the kind of readout `quota` would be: `context_cost.go` computes an estimated-token advisory for `dotagents doctor`, prints it as a note, and explicitly never lets it affect exit codes. Its own correctness comment states the design rule directly: "we deliberately never show a percentage of a context window and never hardcode a model context size... a wrong percentage would be worse than no percentage. Absolute estimates only." That rule transfers directly to quota: show absolute remaining windows and reset times per provider (matching what `agent-quota.py` already does with its `"94% left (resets 3h4m)"` format), and do not invent a single normalized "you have X% left across providers" number, since Claude, Codex, and Gemini have different window shapes and reset semantics.

**How it could work in dotagents**: this is the one that needs an actual product decision, not just an implementation plan, because dotagents' `README.md` capability matrix is a load-bearing honesty claim ("a surface gets a yes only after its native behavior is verified end to end"). Two shapes:
- Minimal: a `dotagents quota` command (Go, no new managed surface, same read-only-advisory shape as `context_cost.go`) that reads Claude's rate-limits cache and Codex's rollout tail, equivalent to `agent-quota.py`, and prints per-provider absolute windows and reset times. No hook/statusline wiring, no `dotagents.yaml` schema change, no capability-matrix claim. This is closest to what `USER.md` already asked for (`/quota`-like, provider-agnostic).
- Full: wire an actual `statusLine` entry into `~/.claude/settings.json` as a fifth managed surface (Skills, MCP servers, Hooks, Agent roles, **Statusline**), requiring a new `hookTarget`-equivalent per harness, `status`/`sync`/`doctor` support, a capability-matrix row, and, since only Claude Code has a native statusLine concept among dotagents' harnesses (Codex/Hermes/Droid do not), an honest single-`yes` row instead of the multi-harness pattern the rest of the matrix follows.

**Effort**: minimal shape is small (a read-only Go command, no config schema, no sync/doctor involvement, ports two Python scripts' logic). Full shape is medium-to-large (new managed-surface category, one honest capability-matrix row, ongoing maintenance as Claude Code's `rate_limits` stdin schema changes across versions; anywhere-agents' own comment notes this is a v2.1.80+ feature that can silently regress).

**Public or private**: the minimal `dotagents quota` command is publicly defensible infrastructure: it is provider-agnostic by construction (reads two on-disk formats, no personal config), matches an already-articulated user want, and doesn't touch the charter's four managed surfaces. Recommend starting there, not with the statusLine-wiring version, and revisiting the full version only if the minimal command turns out to be used enough to justify a fifth managed surface.

### 2d. Dual-agent review loop (`implement-review`)

anywhere-agents' most-developed skill (`implement-review`) is a review loop: detect content type, send staged changes to a second reviewer (Codex by default), categorize feedback, revise, iterate, with an optional Phase-0 plan-review before execution on high-blast-radius work.

**Checked for overlap with dotagents' own `review.go`/`review_apply.go`/`review_test.go`: there is none.** dotagents' `review` command is the interactive setup import-review TUI, the share/keep/skip screen described in `README.md`'s "review screen" section (`reviewAction`, `reviewRow`, `reviewDecision` in `review.go` model a per-item share/keep/skip decision across detected harness content during `setup`). It has nothing to do with code review or a second-opinion agent loop. The name collision (`review`) is coincidental.

So this is a genuine gap, not a duplicate: dotagents has no equivalent to a cross-agent code/diff review workflow. It is also, however, workflow content rather than sync infrastructure, closer in kind to the existing `skills/grilling` starter skill than to a CLI capability. If it is built, it belongs as a skill (`SKILL.md` plus scripts), not as a new dotagents subcommand, and the skill would need to be harness-agnostic by construction (the "Agent Fungibility" principle in anywhere-agents' own `AGENTS.md`, primary implementer down/reviewer down must both remain workable) since dotagents' whole premise is multi-harness parity, more so than anywhere-agents' Claude-primary/Codex-gatekeeper default.

**Effort**: medium. Not a hook or a config schema change, but a real skill with real content: reviewer dispatch logic, a save-contract for review output, categorization rules.

**Public or private**: could go either way. As a generic multi-harness review-loop skill (no baked-in agent preference, configurable primary/reviewer), it is legitimate public starter content, similar in spirit to `skills/grilling`. As tuned to this user's specific review preferences and cadence, it belongs in `~/.agents`. Recommend prototyping privately first; promote via `dotagents skill promote` only once the shape is stable and genuinely harness-agnostic.

## 3. Strategic assessment

### What anywhere-agents does better

- **A working answer to "is my quota running out."** dotagents has nothing here; anywhere-agents ships it as the single most concretely useful daily-driver feature in the whole repo.
- **Generated-file protection idiom.** The `GENERATED FILE` marker plus preserve-and-warn-loudly pattern in `generate_agent_configs.py` is a genuinely good idempotency contract dotagents' `RootInstructionsCapability` doesn't currently need (it's a symlink, not a generated file) but would need the moment 2b(b) above is built.
- **Concrete, testable reroute hints in deny messages.** `guard.py`'s `Suggested rewrite:` line on every deny (compound-cd, writing-style) is a real usability improvement over a bare deny: the agent can self-correct in one turn instead of guessing. dotagents' hook system has no equivalent concept today (its hooks are lifecycle commands, not permission-decision deny/ask hooks at all).
- **A stated, load-bearing product opinion about what NOT to do**, for example "Session checks report, not fix." dotagents doesn't have an equivalent explicit non-goal statement about agent autonomy; it's implicit in the architecture instead.

### What dotagents does better

- **Commit-pinned external content with an explicit update step.** `dotagents.lock` plus `skill update` versus anywhere-agents' SessionStart hook that re-bootstraps (fetches from Git) on every session start. dotagents' model has no unpinned supply chain running on every session; anywhere-agents' bootstrap-every-session design means a compromised or broken upstream is live in every consumer session immediately, with no local pin to fall back to. This is a meaningful security/reliability difference, not a style preference.
- **Multi-harness breadth with a verified capability matrix.** anywhere-agents is explicitly Claude Code + Codex only ("Not a universal multi-agent sync tool," per its own README). dotagents covers eight harnesses with per-surface verification discipline ("Do not add a surface without a verified native adapter and focused tests").
- **Config-root separation.** dotagents never touches project-directory config discovery; anywhere-agents' bootstrap explicitly overwrites the consumer repo's root `AGENTS.md` on every run, forcing all local customization into `AGENTS.local.md`. dotagents' "never restore project-directory config discovery" charter line is a stronger, more predictable contract for a tool meant to run across many repos the user doesn't fully control.
- **No coercive control-flow hooks.** dotagents' hooks are lifecycle commands (session start/end/stop), not PreToolUse deny gates used to force specific model output. See anti-patterns below for why this matters.

### "Make dotagents great again": charter widen or stay narrow?

The user's framing ("dotagents' charter forbids the interesting stuff") is not quite right, and the distinction matters for what to build next. Re-reading `AGENTS.md`'s bans, they split into two different kinds:

1. **Architectural bans that protect the product's central claim**: no project-directory config discovery, "a surface gets a yes only after its native behavior is verified end to end." These exist to keep the capability matrix trustworthy: the thing that makes dotagents worth using over hand-copying config. (The former ban on plugin packaging was lifted in PR #135 to support agent-plugins-spec v1.0.0.)
2. **Inventory bans that are a procedure, not a prohibition**: "New starter content requires an explicit product decision and an inventory test update." That's one commit plus one test update (see `readme_inventory_test.go`, which enforces that the public skill inventory in `README.md` matches what actually ships), not a closed door.

And the discriminating fact: **dotagents already ships executable hook payloads as public content** (`memory/hooks/*.py`, `memory/lib/*.py`; both are named directly in the `//go:embed` directive in `starter_assets.go` alongside `AGENTS.md`, `dotagents.yaml`, `agents/*.md`, and the two starter skills, and `AGENTS.md` explicitly calls this out as intentional public infrastructure). So "a writing-style guard hook" is not a new *category* of thing for the public repo; it's the same category as something already there, shipped through the same embed-and-test-inventory mechanism. The charter doesn't forbid hooks, skills, or interesting mechanisms; it forbids *personal* content (this user's specific word list, this user's SOUL.md) and *unverified* surfaces (a statusline claim with only one working harness).

Recommendation: don't widen the charter. Use the inventory-ban procedure it already provides, and note that this repo's own in-flight `session_hub.go` work is already following that procedure by adding a fifth harness capability outside skills/MCP/hooks/roles. Concretely, in priority order:

1. **Ship `dotagents quota`** (2c minimal shape) as new public infrastructure: provider-agnostic, no personal config baked in, matches a want the user has already stated independent of this analysis. This is the single highest-leverage "make it compelling again" move: it's the one anywhere-agents feature that has no dotagents equivalent at all and solves a real daily annoyance (tight Claude Pro limits, tracking OpenAI/Anthropic/Gemini quota across three subscriptions).
2. **Extend `RootInstructionsCapability` to Claude Code and Codex** (2b-a): small, uses an existing verified mechanism, closes a real gap (only Droid gets synced root instructions today), and is the kind of "boring but real" infrastructure improvement that makes the multi-harness story actually multi-harness instead of Droid-plus-partial.
3. **A generic, config-driven PreToolUse-deny hook capability** in `hookConfig` (not a specific word list, not a specific banner): if built as infrastructure (deny/ask decision types, a word-list or pattern field supplied via `dotagents.yaml`), this is legitimately public, a mechanism, not the user's personal style rules. Ship it capability-first; let `~/.agents` supply the actual list. This also directly serves the private myagents drift problem: the user wouldn't need to leave dotagents to get a style guard if dotagents exposed the primitive and myagents supplied the config.
4. Do **not** build the tagged-block `AGENTS.md` generator (2b-b), the full statusLine-wiring shape (2c full), or the review-loop skill (2d) until 1 to 3 are shipped and used. All three are medium-to-large net-new mechanisms competing for the same "make it interesting again" motivation; the smaller wins first.

The deeper answer to "why did the user drift to myagents": not because the charter is too narrow, but because dotagents currently has zero features that are *fun to use day to day*; it's pure sync-and-verify plumbing. anywhere-agents' quota display and writing guard are things a user notices and benefits from every single session. dotagents' `doctor`/`status`/`sync` are things a user runs and forgets. Shipping items 1 and 3 above (both legitimately public, both mechanism-not-personal-content) is the fastest path to giving dotagents a similar "I notice this working" feature without violating anything the charter actually protects.

## 4. Anti-patterns to avoid

1. **The session-banner gate as PreToolUse deny.** `check_banner_emission` in `guard.py` denies almost every tool call (everything except a short exempt list) until the model emits a fixed-format banner and writes an acknowledgment file. This is a deny-hook used as control flow to coerce specific model output, not to gate a genuinely risky action. It already needed a documented retreat: `check_banner_emission`'s "Re-arm semantics" branch downgrades a second SessionStart to advisory pass-through because the strict version caused reported problems (anywhere-agents issue #7, referenced directly in the code comment). Don't copy this pattern into dotagents' hook system; if dotagents ever adds deny-capable hooks, reserve deny for things that are actually reversible-cost-if-wrong (a banned word, a destructive git command), never for coercing conversational output shape.

2. **Bootstrap overwrites the consumer's root `AGENTS.md` on every session.** anywhere-agents' own docs state this plainly and instruct users to put all local customization in `AGENTS.local.md` because the generated file "will be overwritten." dotagents' `RootInstructionsCapability` is a symlink to a canonical file the user owns and edits directly, no overwrite-and-warn dance needed because there's only one copy. Keep it that way; don't add an overwrite-and-regenerate step to chase feature parity with anywhere-agents' generator.

3. **Network-fetch-and-reapply on every session start.** anywhere-agents' SessionStart hook re-runs bootstrap (a Git sparse-clone fetch) every session. That's session-start latency plus an effectively-unpinned supply chain running automatically, session after session, with no local commit to fall back to if upstream breaks or is compromised mid-session. dotagents' `dotagents.lock` plus explicit `skill update` is strictly better here (see Section 3); don't regress it by adding an auto-fetch-on-every-session-start feature in the name of parity.

4. **`guard.py`'s complexity as the cost of doing PreToolUse security in a hook.** The file is roughly 1500 lines and its own comments document multiple rounds of bypass-and-patch cycles for the trusted-script auto-allow alone (newline smuggling, `$()`-in-double-quoted-path expansion, redirection passthrough; "Round 1," "Round 2," "Round 3" are literally named in the comments). That's the real cost curve of putting a security boundary inside a regex-based hook: every clever bypass needs a new patch, forever. If dotagents builds any deny/ask-capable hook (writing-style guard, destructive-command guard), keep the matching logic minimal and narrow-scoped (a bounded word list, an exact-token classifier) rather than trying to be a general-purpose shell-command risk classifier with wrapper-piercing to arbitrary depth; that generality is exactly what drove guard.py's size and bypass-patch cycle.

5. **The pack-management CLI's version-compatibility surface** (`pack verify --fix`, `pack update` kept as "compatibility aliases through all v0.x," `ANYWHERE_AGENTS_UPDATE=skip` versus `--no-apply-drift` with documented precedence rules) is a lot of surface area for a single-maintainer tool to carry indefinitely. dotagents' smaller, more opinionated command surface (`setup`/`status`/`sync`/`doctor`/`skill`/`mcp`, with `help --all` for the rest) is the right tradeoff for a tool this user maintains solo; don't grow a parallel pack-CLI-with-legacy-alias-matrix chasing anywhere-agents' surface.

---

## Key file references

anywhere-agents:
- `/Users/conscience/Public/anywhere-agents/scripts/guard.py`
- `/Users/conscience/Public/anywhere-agents/scripts/statusline.py`
- `/Users/conscience/Public/anywhere-agents/scripts/agent-quota.py`
- `/Users/conscience/Public/anywhere-agents/scripts/generate_agent_configs.py`
- `/Users/conscience/Public/anywhere-agents/AGENTS.md`
- `/Users/conscience/Public/anywhere-agents/bootstrap/bootstrap.sh`
- `/Users/conscience/Public/anywhere-agents/bootstrap/packs.yaml`
- `/Users/conscience/Public/anywhere-agents/user/settings.json`
- `/Users/conscience/Public/anywhere-agents/.claude/settings.json`
- `/Users/conscience/Public/anywhere-agents/skills/{ci-mockup-figure,implement-review,my-router,readme-polish}/SKILL.md`

dotagents:
- `/Users/conscience/Workspace/dotagents/cmd/dotagents/hooks.go`
- `/Users/conscience/Workspace/dotagents/cmd/dotagents/harness.go`
- `/Users/conscience/Workspace/dotagents/cmd/dotagents/inspect.go`
- `/Users/conscience/Workspace/dotagents/cmd/dotagents/sync.go`
- `/Users/conscience/Workspace/dotagents/cmd/dotagents/root_instructions_test.go`
- `/Users/conscience/Workspace/dotagents/cmd/dotagents/context_cost.go`
- `/Users/conscience/Workspace/dotagents/cmd/dotagents/session_hub.go` (untracked, in-flight work; cited in Section 3 as precedent for adding a fifth capability)
- `/Users/conscience/Workspace/dotagents/cmd/dotagents/review.go` (setup import-review TUI; no overlap with anywhere-agents' `implement-review` code-review skill, see 2d)
- `/Users/conscience/Workspace/dotagents/starter_assets.go`
- `/Users/conscience/Workspace/dotagents/AGENTS.md`
- `/Users/conscience/Workspace/dotagents/README.md`
