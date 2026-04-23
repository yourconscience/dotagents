---
name: dotagents
description: Inspect and sync the repo-owned agent skill links for the dotagents repo across supported coding agents. Use when the user asks for dotagents status, dotagents sync, dotagents setup, or wants to reconcile ~/.agents with Claude/Codex/Hermes/OpenClaw skill roots.
---

# dotagents

Use the skill-local CLI tool. This skill is specific to the `~/Workspace/dotagents` repo and its managed symlink surface.

## Commands

From the repo root:

```bash
go run ./skills/dotagents/tools/dotagents setup                # first-time machine setup
go run ./skills/dotagents/tools/dotagents status               # show sync state
go run ./skills/dotagents/tools/dotagents sync                 # sync skill symlinks
go run ./skills/dotagents/tools/dotagents pull                 # git pull + sync (for cron)
go run ./skills/dotagents/tools/dotagents cron --interval 30m  # install auto-pull crontab
go run ./skills/dotagents/tools/dotagents cron --remove        # remove crontab entry
```

Limit a run to specific agents:

```bash
go run ./skills/dotagents/tools/dotagents sync --agents=claude-code,hermes
```

## Subcommands

### setup

First-time setup on a new machine. Does three things:
1. Creates `~/.agents` symlink pointing at the repo root.
2. Patches detected agent configs to load skills from `~/.agents/skills` (Hermes: adds to `skills.external_dirs` in config.yaml; OpenClaw: adds to `skills.load.extraDirs` in openclaw.json; Claude Code/Codex: handled by symlinks, no patching needed).
3. Runs `sync` for agents that use managed skill-root symlinks.

Important Hermes note: Hermes should consume dotagents skills primarily via `skills.external_dirs: ["~/.agents/skills"]`, not by mirroring repo skills into `~/.hermes/skills`. Hermes already ships a bundled categorized skill tree under `~/.hermes/skills`, so symlinking repo skills there can collide with bundled category directories. Example: a repo skill named `research` conflicts with Hermes' builtin `research/` category. Prefer Hermes-native bundled skills when an equivalent already exists there. Example: use Hermes' builtin `google-workspace` skill instead of trying to override it from dotagents. Treat `external_dirs` as the canonical Hermes integration path.

When a dotagents skill overlaps with a Hermes builtin skill, prefer the Hermes builtin and let the external dotagents copy exist only for other agents. Current example: the shared `skills/gws/` skill now has frontmatter name `google-workspace`, but on Hermes the builtin `google-workspace` skill should win and no extra sync action is needed.

### status

Reports sync state for each detected agent. Agents whose binary is not on PATH show "not detected" and are skipped.

### sync

Creates, updates, or removes skill symlinks in each detected agent's skill root for agents that use managed mirrors. Non-repo skills are reported as `external` and left untouched. Hermes is special-cased: `sync` verifies the `skills.external_dirs` integration and does not try to mirror repo skills into `~/.hermes/skills`.

### pull

Runs `git pull --ff-only` in the repo root, then `sync`. Designed to be called from cron for auto-sync on remote machines.

### cron

Installs or removes a crontab entry that runs `pull` on a schedule. Default interval is 30m. Options: 5m, 15m, 30m, 1h, 6h, 12h, daily.

### promote

Promotes a Hermes skill to dotagents shared skills. Copies the skill, creates a branch, commits, pushes, and opens a PR.

```bash
go run ./skills/dotagents/tools/dotagents promote <skill-name>        # by name (searches ~/.hermes/skills/)
go run ./skills/dotagents/tools/dotagents promote <category>/<name>   # with category prefix
go run ./skills/dotagents/tools/dotagents promote <name> --dry-run    # copy only, skip git/PR
```

The skill is searched in `~/.hermes/skills/` by name (including inside category subdirectories). The promote command:
1. Copies the skill to `skills/<name>/`
2. Creates branch `promote/<name>`
3. Commits as "add <name> skill"
4. Pushes and creates a PR via `gh`

Requires `gh` CLI authenticated with push access to the repo.

For the full promotion workflow (evaluation, pr-triage, merge), use the `skill-promote` Hermes skill.

## What it manages

- Fixes `~/.agents` if it should point at the repo root and is missing or drifted.
- Detects agents by checking if their binary is on PATH (`detect` field in config).
- Treats repo skills under `skills/` as the managed set for each detected agent.
- Reports non-repo skills already present in agent skill roots as `external` and leaves them untouched.
- Stops on conflicts when a managed skill path exists as a real file or directory instead of a symlink.

## Config

Default targets live in:

```bash
skills/dotagents/dotagents.yaml
```

Use `--agents` only as a one-run override.
