# dotagents

Dotfiles for your AI agents. One `~/.agents` repo holds your skills, MCP servers, hooks, and subagent roles — dotagents syncs it into Claude Code, Codex, Droid, Hermes, and Pi. External skills are commit-pinned in `dotagents.lock` and audited for risky patterns.

This is *your* agent setup, not a project's: it lives in your home directory, versions like dotfiles, and follows you across machines and harnesses. Project-level `AGENTS.md` files stay in their repos where they belong.

**Landing page:** [yourconscience.github.io/dotagents](https://yourconscience.github.io/dotagents/) · **Releases:** [v0.1.0](https://github.com/yourconscience/dotagents/releases/latest)

![dotagents harness map](./docs/harness-map.png)

## Install

**Plugin only** (Claude Code or Codex, no CLI needed):

```
/plugin marketplace add yourconscience/dotagents
/plugin install dotagents@yourconscience
```

**Full setup** (all harnesses):

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
| Pi/OMP | yes | -- | yes | -- |

Amp, OpenCode, and OpenClaw are compatibility-only: they can read the repo's skills via standard conventions, but dotagents does not manage them.

## Skills

18 skills ship with this repo:

`spawn` `cmux` `tmux` `remote-access` `repo-eval` `review` `tech-search` `x-sim` `grill-me` `humanizer` `spec` `jobs` `pr-triage` `gws` `tg` `x-cli` `spotify` `dotagents`

A skill is a `SKILL.md` in a directory under `skills/`. Add one, run `dotagents sync`, it shows up everywhere.

## Agent roles

Four roles defined in `agents/subagents.yaml`, rendered to each harness's native format:

`architect` `builder` `researcher` `reviewer`

## Key commands

```bash
dotagents status          # what's synced where
dotagents sync            # push changes to all harnesses
dotagents doctor          # validate config, check for drift
dotagents render          # regenerate plugin copies and agent files
dotagents plugin add      # switch Claude Code to plugin delivery
dotagents deps check      # verify external tool dependencies
```

## External skills

Pull skills from other repos:

```yaml
# dotagents.yaml
external_skills:
  - url: https://github.com/example/shared-skills
    skill_dir: skill
    branch: main
    skills: [alpha, beta]
```

Sources are pinned in `dotagents.lock`. `dotagents doctor` audits external skills for risky patterns.

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
| Harness coverage | 5 deep | 40+ broad | 35+ broad | Claude-family + majors |
| Install | plugin, curl, or `go install` | npm, brew, binary | npm | npm |

rulesync and ruler are project-level: they generate per-tool config files inside a repo and win on tool breadth. dotagents is user-level: your whole agent setup — settings, skills, MCP, hooks, roles — lives in one versioned `~/.agents` repo that follows you across machines, syncs into each harness's native surface, and pins external skills to audited commits before they reach any agent. Sync less, deliberately: skills can be scoped per harness so each agent carries only what it needs.
