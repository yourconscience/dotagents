# SPEC

## Goal

Separate the public `yourconscience/dotagents` Go CLI repository from the user's private `~/.agents` configuration repository. The public repository must be a reusable tool distribution; `~/.agents` must be the user-owned canonical store that the CLI reads and syncs into detected harnesses.

## Non-goals

- No `init` command; `setup` owns first-run scaffolding.
- No automated migration for the existing maintainer checkout.
- No plugin packaging, plugin delivery mode, or native-plugin projection.
- No project-local config discovery or CWD walking.
- No semantic merge of conflicting imported content in v1.

## User story / behavior

1. On a new machine, `dotagents setup` creates `$DOTAGENTS_HOME` or `~/.agents`, detects supported harnesses, writes a working `dotagents.yaml`, optionally imports existing native skills, roles, and MCP servers without modifying their sources, selects a memory tier, and runs the first sync.
2. On an existing installation, `setup` reuses the canonical home, patches detected harness configuration, offers imports, and syncs.
3. `dotagents` resolves config from `--config` when provided; otherwise from `$DOTAGENTS_HOME/dotagents.yaml`; otherwise from `~/.agents/dotagents.yaml`. It never discovers project-local config.
4. `sync`, `status`, and `doctor` manage exactly four surfaces: skills, MCP servers, hooks, and agent roles.
5. Imported duplicate names are resolved interactively as keep-left, keep-right, or skip; imported files are copied/converted and originals remain untouched.
6. `setup --memory off|basic|memsearch` controls memory hooks. `basic` is the default, requires only Python 3, appends session digests under `$KNOWLEDGE_DIR/sessions/`, and injects recent entries at session start. `memsearch` preserves the indexed workflow; `off` registers no memory hooks.

## Acceptance tests

- A fake-home E2E with existing Claude Code and Codex native content demonstrates detect → scan → prompt/import → sync → status, including Codex TOML role conversion and MCP JSON-to-YAML conversion, while source files remain byte-identical.
- First-run setup creates the canonical home and config, records detected agents, defaults to basic memory, and completes sync.
- Config tests prove precedence `--config` > `DOTAGENTS_HOME` > `~/.agents` and prove a CWD `dotagents.yaml` is ignored.
- Memory tests prove off registers no hooks; basic session-end appends a dated markdown digest and session-start emits recent context; memsearch retains current hook behavior and dependency checks.
- Public-repository inventory tests prove only `skills/dotagents` and `skills/grilling` ship, five generic roles ship, plugin packages/code/config are absent, and the template is minimal.
- Focused and full Go tests pass; the isolated setup E2E passes; help, setup, sync, status, and doctor smoke tests match the documented surface.
- Independent memory review reports no unresolved correctness, privacy, portability, or data-loss finding before E2E approval.
- The implementation PR has passing checks, no unresolved active review comments, and is merged only after the post-open inspection and review wait.

## Constraints

- Public template:
  - `version: 1`
  - `agents: []`, populated by setup
  - one materialized, pinned `mattpocock/skills` grilling external
  - working memory hook examples
- Public repository retains CLI source, docs, release metadata, memory infrastructure, `dotagents` and `grilling` skills, and architect/builder/researcher/reviewer/tester roles.
- The private repository retains personal skills, agents, hooks, MCP configuration, memory overrides, lock state, and external cache.
- Import is copy/convert, never move. After acceptance, canonical `~/.agents` content owns future sync output.
- Existing non-managed native content remains untouched.
- Dates and user-facing diagnostics are deterministic and tests use isolated temporary homes.

## Dependencies / integrations

- Go CLI and existing harness adapters.
- Python 3 for basic memory.
- `memsearch` only for the memsearch tier.
- Git for external skill pinning and materialization.
- GitHub pull-request checks and review threads for publication.

## Risks / open questions

- Native files already managed by the old checkout can look like import candidates; setup must identify or safely resolve conflicts without deleting originals.
- MCP secrets must remain environment references; imports must not expose secret values.
- Basic memory must avoid recording raw transcript data beyond the existing digest contract and must tolerate an unset knowledge directory with an actionable error.
- Redirecting the maintainer's live `~/.agents` symlink is a manual cutover and must happen only after private content is preserved and the new CLI passes isolated E2E.

## Codebase notes

The confirmed design is `docs/plans/launch-2026-07-14/06-separation-design.md`. It supersedes plugin-specific launch assumptions in earlier launch-prep documents. The user's request and the design's “all decisions confirmed” record constitute approval to implement this specification.

## Outcome / Deviations

Implemented the clean public/private cutover. The public repository now ships the Go CLI, two starter skills, five Markdown roles, minimal config/lock templates, and memory infrastructure; personal skills, MCP declarations, hooks, and local state live in the private configuration repository.

`setup` now resolves only the user-level canonical root, detects harnesses, imports skills/roles/MCP entries, applies `off`, `basic`, or `memsearch` memory, scaffolds embedded starter assets, and runs sync. Plugin delivery and the personal source catalog were removed. Imported native directories whose complete content matches canonical content remain intact and count as synchronized; divergent copies remain conflicts. Literal MCP argument secrets, nested symlinks, and special files are rejected rather than copied into the canonical repository.

Verification completed with the full Go tests, focused memory tests, repeated concurrent-memory tests, an existing-skill setup → import → sync → status E2E, a custom-root memsearch E2E, and an independent memory review with no remaining Critical, High, or Medium finding.
