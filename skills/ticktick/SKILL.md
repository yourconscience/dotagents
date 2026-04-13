---
name: ticktick
description: TickTick task management - list tasks, projects, and notes via CLI.
---

# ticktick

CLI for TickTick task management. Wraps ticktick-sdk Python library.

## Prerequisites

Install ticktick-sdk from PyPI, or point to a local checkout via `TICKTICK_SDK_PATH`:

```bash
# Option A: from PyPI (recommended)
uv pip install ticktick-sdk

# Option B: from local checkout
export TICKTICK_SDK_PATH=~/Public/ticktick-sdk
```

Required environment variables (set in shell or .env):
```
TICKTICK_CLIENT_ID=...
TICKTICK_CLIENT_SECRET=...
TICKTICK_ACCESS_TOKEN=...
TICKTICK_USERNAME=...
TICKTICK_PASSWORD=...
```

## Auth

First-time setup:
```bash
uv run --with ticktick-sdk ticktick-sdk auth
```

This opens browser for OAuth2 flow and prints the access token to add to env.

## Usage

All commands run the tool from the skill directory. Set `TICKTICK_SDK_PATH` if using a local checkout, otherwise the PyPI package is used:

```bash
# Helper: define once per shell
TT="uv run --with ${TICKTICK_SDK_PATH:-ticktick-sdk} python ~/.agents/skills/ticktick/tools/tt.py"

# List all tasks (returns all incomplete tasks across projects)
$TT tasks

# List tasks with filters
$TT tasks --project "Work" --status incomplete

# List all projects
$TT projects

# List tasks in a specific project
$TT tasks --project-id <project_id>

# Export tasks as JSON
$TT tasks --json

# Search tasks by keyword
$TT search "keyword"
```

## Output Formats

- Default: Human-readable table
- `--json`: JSON output for programmatic use
- `--markdown`: Markdown table

## Agent Workflow

When user asks about tasks:
1. Run `tt.py tasks` to get current task list
2. Parse output and summarize for user
3. Do NOT create/edit tasks unless explicitly requested

When user asks to see notes:
1. Notes in TickTick are tasks with `kind: NOTE`
2. Run `tt.py tasks --kind note` to filter notes

## Limitations

- Read-only for now (no create/update/complete)
- Requires OAuth2 setup before first use
- Rate limits may apply (429 errors)
