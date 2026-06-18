# dotagents

Go CLI that keeps skills, MCP servers, hooks, and agent roles in one `~/.agents` repo and syncs them into Claude Code, Codex, Droid, Hermes, and Pi.

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

| Harness | Skills | Roles | MCP | Hooks | Status |
|---|---|---|---|---|---|
| Claude Code | yes | yes | yes | yes | managed |
| Codex | yes | yes | yes | yes | managed |
| Factory Droid | yes | yes | yes | yes | managed |
| Hermes | yes | -- | yes | yes | managed |
| Pi/OMP | yes | -- | yes | -- | managed |
| Amp | -- | -- | -- | -- | compat |
| OpenCode | -- | -- | -- | -- | compat |
| OpenClaw | -- | -- | -- | -- | compat |

## Skills

16 skills ship with this repo:

`spawn` `cmux` `tmux` `remote-access` `repo-eval` `tech-search` `x-sim` `grill-me` `humanizer` `spec` `jobs` `pr-triage` `gws` `tg` `x-cli` `dotagents`

A skill is a `SKILL.md` in a directory under `skills/`. Add one, run `dotagents sync`, it shows up everywhere.

## Agent roles

Four roles defined in `agents/*.yaml`, rendered to each harness's native format:

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

| | dotagents | [gstack](https://github.com/garrytan/gstack) | [SuperClaude](https://github.com/SuperClaude-Org/SuperClaude_Framework) | [skillshare](https://github.com/runkids/skillshare) |
|---|---|---|---|---|
| Skills sync | yes | -- | -- | yes |
| MCP sync | yes | no | no | no |
| Hooks sync | yes | no | yes | no |
| Agent roles | yes | 23 built-in | 20 built-in | no |
| Security audit | yes | no | no | yes |
| Multi-harness | 5 managed + 3 compat | Claude Code | Claude Code | 60+ |
| Install | plugin, curl, or `go install` | CC plugin | pipx | `go install` |

gstack and SuperClaude are content packs for Claude Code. skillshare syncs skills broadly but not MCP, hooks, or roles. dotagents is the full config layer across harnesses.
