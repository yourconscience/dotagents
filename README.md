# dotagents

Dotfiles for your AI agents.


[![Release](https://img.shields.io/github/v/release/yourconscience/dotagents)](https://github.com/yourconscience/dotagents/releases) [![brew](https://img.shields.io/badge/brew-yourconscience%2Ftap-orange)](https://github.com/yourconscience/homebrew-tap) [![npm](https://img.shields.io/npm/v/@your_conscience/dotagents)](https://www.npmjs.com/package/@your_conscience/dotagents) [![CI](https://github.com/yourconscience/dotagents/actions/workflows/ci.yml/badge.svg)](https://github.com/yourconscience/dotagents/actions/workflows/ci.yml) [![License](https://img.shields.io/badge/license-MIT-green)](./LICENSE)

```bash
brew install yourconscience/tap/dotagents
```

> Not affiliated with the unscoped npm `dotagents` package — this repo publishes as [`@your_conscience/dotagents`](https://www.npmjs.com/package/@your_conscience/dotagents).

**[Overview & comparison →](https://yourconscience.github.io/dotagents/)** · [Releases](https://github.com/yourconscience/dotagents/releases) · [Docs](docs/)

## Why

If you use more than one coding agent, you maintain the same skills, MCP servers, hooks, and roles in a different place and format for each one. Copying them by hand drifts within a week. Skills have converged on one open convention ([agentskills.io](https://agentskills.io)), plugins on [agent-plugins-spec](https://agent-plugins.org), and root instructions on `AGENTS.md` — but every harness still stores and renders config in its own native format. dotagents applies the dotfiles pattern to that last mile: one versioned repo, rendered natively per harness, with memory tooling built in.

## Quick start

```bash
brew install yourconscience/tap/dotagents   # or: npm i -g @your_conscience/dotagents
dotagents setup                             # detect harnesses, import, first sync
```

`setup` creates `~/.agents`, detects installed harnesses, imports existing content by copy after a per-item review, and runs the first sync. To carry the setup to other machines, add a private git remote and repeat — details in [docs/setup.md](docs/setup.md).

```bash
dotagents status   # per-harness sync state
dotagents doctor   # health checks: frontmatter, lock pins, audits, hooks
```

## What it syncs

Five surfaces, each rendered into the harness's own format — dotagents does not invent compatibility files a harness cannot consume:

| Harness | Skills | Roles | MCP | Hooks | Plugins |
|---|---|---|---|---|---|
| Claude Code | yes | yes | yes | yes | -- |
| Codex | yes | yes | yes | yes | planned |
| Factory Droid | yes | yes | yes | yes | -- |
| Hermes | yes | -- | yes | yes | -- |
| OpenCode | yes† | yes | yes | -- | -- |
| OMP (pi fork) | yes | yes | yes | --‡ | -- |
| Pi* | yes | --* | --* | -- | -- |

\* Vanilla [pi](https://github.com/earendil-works/pi) is skills-only by design; the OMP fork is detected as its own target.
† OpenCode reads `~/.agents/skills/` natively; its only hook surface is a JS plugin API.
‡ OMP has no managed hook surface yet; register memory hooks manually if needed.

Amp and OpenClaw can read the repo's skills through standard conventions but are not managed; a surface gets a "yes" above only after its native behavior is verified end to end.

## Skills

A skill is a directory under `~/.agents/skills/` with a `SKILL.md` ([agentskills.io](https://agentskills.io) convention). Create once, appears everywhere:

```bash
dotagents skill new review-checklist --description "Pre-merge review checklist"
dotagents sync
```

External skills from other people's repos are treated like dependencies: pinned to an exact commit in `dotagents.lock`, materialized into your repo for diffing, audited by `dotagents doctor`. See [docs/skills.md](docs/skills.md).

## Memory

Pick a tier during `setup`: `off`, `basic` (session digests), or `memsearch` (indexed search). On top of that, `sync` builds two Go helpers into `~/.local/bin`: `knowledge-sync` (vault git sync) and `rem`:

```bash
rem add -src claude "prefers pnpm for Node work"   # capture a candidate fact anywhere
rem dream                                          # consolidate candidates into review reports
rem dream --apply                                  # collapse exact-duplicate records (backup + commit)
rem search "quota preferences"                     # semantic search over captured memory
```

Candidates are inert until you promote them into durable instructions — consolidation is report-first by design, because automatically rewriting memory is how agents quietly corrupt their own instructions. Design notes in [docs/memory.md](docs/memory.md).

## Roles

Markdown role definitions in `~/.agents/agents/`, rendered to each harness's native format (Claude Markdown, Codex TOML, Droid). Generic `model` tiers (`haiku`/`sonnet`/`opus`) render natively per family; per-harness overrides pin exact ids. Six starter roles ship with the tool; yours win on name collision. Details in [docs/roles.md](docs/roles.md).

## Commands

```bash
dotagents setup    [--memory off|basic|memsearch] [--yes] [--dry-run] [--json]
dotagents status   [--agents ...]
dotagents sync     [--pull] [--agents ...]
dotagents doctor   [--e2e] [--agents ...]
dotagents skill    new|update|promote
dotagents mcp      list|add|import|remove
```

## Configuration

`~/.agents/dotagents.yaml` is the single source of truth; `setup` fills in detected harnesses. Resolution order: `--config <path>` → `$DOTAGENTS_HOME/dotagents.yaml` → `~/.agents/dotagents.yaml`; never walks the current project. Machine-local entries overlay via `dotagents.local.yaml`. Managed entries are marked in native configs; anything else is left untouched.

## Releases

```bash
scripts/release.sh v0.7.0    # verify + tag; CI publishes binaries, brew tap, npm
```

## Documentation

- [docs/setup.md](docs/setup.md) — first-run walkthrough, review screen, multi-machine setup
- [docs/skills.md](docs/skills.md) — authoring skills, external pins and audits
- [docs/roles.md](docs/roles.md) — role format, model tiers, per-harness overrides
- [docs/memory.md](docs/memory.md) — memory tiers, rem workflow, vault layout
- [docs/comparison.md](docs/comparison.md) — how dotagents differs from rulesync, ruler, openskills
- [Troubleshooting](docs/troubleshooting.md)

## How it differs

Project-level generators (rulesync, ruler) win on tool breadth; dotagents is user-level: one private repo that follows you across machines, seven targets deep, with pinned and audited external skills and a review-first memory workflow. Full table in [docs/comparison.md](docs/comparison.md).

## License

[MIT](./LICENSE)
