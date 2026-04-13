---
name: ticktick
description: TickTick task management - list tasks, projects, and notes via CLI.
---

# ticktick

CLI for TickTick task management. Wraps ticktick-sdk Python library.

## Prerequisites

The SDK is installed from local clone at `~/Public/ticktick-sdk`.

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
cd ~/Public/ticktick-sdk && uv run ticktick-sdk auth
```

This opens browser for OAuth2 flow and prints the access token to add to env.

## Usage

All commands run from the skill tools directory:

```bash
# List all tasks (default: today + overdue)
uv run --with ~/Public/ticktick-sdk python ~/.agents/skills/ticktick/tools/tt.py tasks

# List tasks with filters
uv run --with ~/Public/ticktick-sdk python ~/.agents/skills/ticktick/tools/tt.py tasks --project "Work" --status incomplete

# List all projects
uv run --with ~/Public/ticktick-sdk python ~/.agents/skills/ticktick/tools/tt.py projects

# List tasks in a specific project
uv run --with ~/Public/ticktick-sdk python ~/.agents/skills/ticktick/tools/tt.py tasks --project-id <project_id>

# Export tasks as JSON
uv run --with ~/Public/ticktick-sdk python ~/.agents/skills/ticktick/tools/tt.py tasks --json

# Search tasks by keyword
uv run --with ~/Public/ticktick-sdk python ~/.agents/skills/ticktick/tools/tt.py search "keyword"
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
