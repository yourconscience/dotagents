---
name: dotagents
description: Inspect and sync the repo-owned agent skill links for the dotagents repo across supported coding agents. Use when the user asks for dotagents status, dotagents sync, or wants to reconcile ~/.agents with Claude/Codex/Hermes/OpenClaw skill roots.
---

# dotagents

Use the skill-local CLI tool. This skill is specific to the `~/Workspace/dotagents` repo and its managed symlink surface.

## Commands

From the repo root:

```bash
go run ./skills/dotagents/tools/dotagents status
go run ./skills/dotagents/tools/dotagents sync
```

Limit a run to specific configured agents:

```bash
go run ./skills/dotagents/tools/dotagents status --agents=codex
go run ./skills/dotagents/tools/dotagents sync --agents=claude-code,codex
```

## What it manages

- Fixes `~/.agents` if it should point at `~/Workspace/dotagents` and is missing or drifted.
- Treats repo skills under `skills/` as the managed set for each selected agent.
- Reports non-repo skills already present in agent skill roots as `external` and leaves them untouched.
- Stops on conflicts when a managed skill path exists as a real file or directory instead of a symlink.

## Config

Default targets live in:

```bash
skills/dotagents/dotagents.yaml
```

Use `--agents` only as a one-run override.
