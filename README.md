# dotagents

Dotfiles for your AI agents. One `~/.agents` repo holds your skills, MCP servers, hooks, and subagent roles. dotagents syncs each supported capability into Claude Code, Codex, Droid, Hermes, Pi, and OMP. External Git skills are commit-pinned in `dotagents.lock` and audited for risky patterns.

This is *your* agent setup, not a project's: it lives in your home directory, versions like dotfiles, and follows you across machines and harnesses. Project-level `AGENTS.md` files stay in their repos where they belong.

**Landing page:** [yourconscience.github.io/dotagents](https://yourconscience.github.io/dotagents/) · **Releases:** [v0.1.0](https://github.com/yourconscience/dotagents/releases/latest)

![dotagents harness map](./docs/harness-map.png)

## Install

**Claude Code plugin only** (portable skills, no CLI needed):

```
/plugin marketplace add yourconscience/dotagents
/plugin install dotagents@yourconscience
```

**Codex plugin only** (portable skills, no CLI needed):

```bash
codex plugin marketplace add yourconscience/dotagents
codex plugin add dotagents@yourconscience
```

Codex installs the zero-copy package rooted at `skills/`: `.agents/plugins/marketplace.json` selects `./skills`, and `skills/.codex-plugin/plugin.json` exposes the canonical tree directly. No rendered skill copy is committed.

**Full sync setup** (all supported harnesses):

```bash
# curl installer (macOS/Linux, no Go required)
curl -fsSL https://raw.githubusercontent.com/yourconscience/dotagents/main/scripts/install.sh | sh

# or via mise
mise use -g github:yourconscience/dotagents

# or with Go
go install github.com/yourconscience/dotagents/cmd/dotagents@latest
```

Then clone and initialize:

```bash
git clone https://github.com/yourconscience/dotagents ~/.agents
dotagents setup
```

Prebuilt binaries for macOS and Linux (amd64/arm64) on [Releases](https://github.com/yourconscience/dotagents/releases).

## What it does

| Harness | Skills | Roles | MCP | Hooks |
|---|---|---|---|---|
| Claude Code | yes | yes | yes | yes |
| Codex | yes | yes | yes | yes |
| Factory Droid | yes | yes | yes | yes |
| Hermes | yes | -- | yes | yes |
| Pi | yes | -- | -- | -- |
| OMP | yes | yes | yes | -- |

Amp, OpenCode, and OpenClaw are compatibility-only: they can read the repo's skills via standard conventions, but dotagents does not manage them.

## Skills

<!-- BEGIN GENERATED SKILLS -->
20 skills ship with this repo:

`cmux` `domain-modeling` `dotagents` `grilling` `gws` `humanizer` `jobs` `pr-triage` `remote-access` `repo-eval` `review` `spawn` `spec` `spotify` `tech-search` `tg` `tmux` `wayfinder` `x-cli` `x-sim`
<!-- END GENERATED SKILLS -->

A skill is a `SKILL.md` in a directory under `skills/`. Add one and run `dotagents sync` to expose it through each configured skill surface. Skills projected from installed native plugins are discovered from those installations; unlike `external_skills` Git sources, they are not pinned in `dotagents.lock` or audited as external repositories.

## Agent roles

Four roles defined in `agents/subagents.yaml`, synced to each harness that supports native roles:

`architect` `builder` `researcher` `reviewer`

## Key commands

```bash
dotagents setup [--delivery plugin|sync] [--agents ...]
dotagents status [--agents ...]
dotagents sync [--pull] [--agents ...]
dotagents doctor [--e2e] [--agents ...]
dotagents skill new <name> [--description ...]
dotagents skill update [name ...]
dotagents skill promote <name-or-path> [--dry-run]
dotagents mcp <list|add|import|remove> [options]
dotagents cron [--interval 30m|--deps|--remove]
```

Run `dotagents help --all` for maintenance commands and explicitly labeled compatibility aliases.

## External skills

Pull and pin a Git skill library once, selecting paths from any upstream subdirectory:

```yaml
# dotagents.yaml
external_skills:
  - url: https://github.com/example/shared-skills
    branch: main
    skill_dirs: [engineering/alpha, productivity/beta]
    materialize: true
```

`materialize: true` makes `dotagents sync` copy each selected tree exactly into canonical `skills/<basename>` and prune stale files. Harnesses then receive only `~/.agents/skills` paths. `dotagents.lock` pins the source commit and records ownership so `doctor` can detect copy drift or direct cache delivery. `dotagents skill update [name ...]` explicitly advances pins and refreshes those copies immediately. Existing `skill_dir` plus optional `skills` entries remain supported and default to direct delivery from the shared `~/.agents/external` cache.

`dotagents doctor` scans external sources for risky patterns. Installed native-plugin skill surfaces are projected separately and are not represented as pinned Git libraries.

Private additions go in `dotagents.local.yaml` (gitignored).

## Landscape

| | dotagents | [rulesync](https://github.com/dyoshikawa/rulesync) | [ruler](https://github.com/intellectronica/ruler) | [openskills](https://github.com/numman-ali/openskills) |
|---|---|---|---|---|
| Source of truth | canonical git repo (`~/.agents`) | unified files → generated configs | `.ruler/` dir → applied configs | skills installed from repos |
| Skills sync | yes | yes | yes | yes |
| MCP sync | yes | yes | yes | no |
| Hooks sync | yes | yes | no | no |
| Subagent roles | yes (`subagents.yaml`) | yes | experimental | no |
| Supply-chain pinning | `dotagents.lock` + `doctor` audit | no | no | no |
| Harness coverage | 6 deep | 40+ broad | 35+ broad | Claude-family + majors |
| Install | plugin, curl, or `go install` | npm, brew, binary | npm | npm |

rulesync and ruler are project-level: they generate per-tool config files inside a repo and win on tool breadth. dotagents is user-level: one versioned `~/.agents` repo follows you across machines and syncs each harness's supported skills, MCP, hooks, and roles into its native surface. External Git skill libraries are pinned to audited commits before delivery; installed native-plugin skill surfaces are projected from the local installation instead. Skills can be scoped per harness so each agent carries only what it needs.
